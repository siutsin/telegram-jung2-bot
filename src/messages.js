const Pino = require('pino')
const moment = require('moment')
const ip = require('ip')
const DynamoDB = require('./dynamodb.js')
const SQS = require('./sqs.js')

function _isBotCommand (message) {
  return message.entities && message.entities[0] && message.entities[0].type === 'bot_command'
}

class Messages {
  constructor () {
    this.dynamodb = new DynamoDB()
    this.logger = new Pino({ level: process.env.LOG_LEVEL })
    this.sqs = new SQS()
  }

  async newMessage (event) {
    this.logger.info(`newMessage start at ${moment().utcOffset(8).format()}`)
    try {
      const xForwardedFor = event.headers['X-Forwarded-For'] || event.headers['x-forwarded-for']
      const requestIP = xForwardedFor.split(', ')[0]
      // TODO: Should use WAF
      if (!ip.cidrSubnet('91.108.4.0/22').contains(requestIP) && !ip.cidrSubnet('149.154.160.0/20').contains(requestIP)) {
        this.logger.info(`Not Telegram Bot IP: ${requestIP}`)
        return { statusCode: 403, message: 'Not Telegram Bot IP' }
      }
      const params = JSON.parse(event.body)
      this.logger.trace(`messages.js::newMessage params: ${JSON.stringify(params)}`)
      const message = params.message
      if (!message || !message.chat.type.includes('group')) {
        // handle edited_message and non group
        return { statusCode: 204, message: 'edited_message or non-group' }
      }
      await this.dynamodb.saveMessage({ message })
      if (_isBotCommand(message)) {
        const text = message.text
        this.logger.info(text)
        if (text.match(/\/jung[hH]elp/)) {
          this.logger.info('newMessage help')
          await this.sqs.sendJungHelpMessage(message)
        }
        if (text.match(/\/top[tT]en/)) {
          this.logger.info('newMessage topTen')
          await this.sqs.sendTopTenMessage(message)
        }
        if (text.match(/\/top[dD]iver/)) {
          this.logger.info('newMessage topDiver')
          await this.sqs.sendTopDiverMessage(message)
        }
        if (text.match(/\/all[jJ]ung/)) {
          this.logger.info('newMessage alljung')
          await this.sqs.sendAllJungMessage(message)
        }
        if (text.match(/\/enable[aA]ll[jJ]ung/)) {
          this.logger.info('newMessage enableAllJung')
          await this.sqs.sendEnableAllJungMessage(message)
        }
        if (text.match(/\/disable[aA]ll[jJ]ung/)) {
          this.logger.info('newMessage disableAllJung')
          await this.sqs.sendDisableAllJungMessage(message)
        }
      }
      this.logger.info(`newMessage finish at ${moment().utcOffset(8).format()}`)
      return { statusCode: 200 }
    } catch (e) {
      this.logger.error(e.message)
      this.logger.info(`newMessage finish with error at ${moment().utcOffset(8).format()}`)
      return { statusCode: 500 }
    }
  }
}

module.exports = Messages
