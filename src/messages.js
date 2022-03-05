const Pino = require('pino')
const moment = require('moment')
const ip = require('ip')
const { DateTime } = require('luxon')

const DynamoDB = require('./dynamodb.js')
const SQS = require('./sqs.js')
const Settings = require('./settings')

class Messages {
  constructor () {
    this.dynamodb = new DynamoDB()
    this.logger = new Pino({ level: process.env.LOG_LEVEL })
    this.sqs = new SQS()
    this.settings = new Settings()
  }

  isBotCommand (message) {
    return message.entities && message.entities[0] && message.entities[0].type === 'bot_command'
  }

  isValidTimezone (tzString) {
    return DateTime.local().setZone(tzString).isValid
  }

  async handleBotCommand (message) {
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
    // setOffFromWorkTimeUTC
    // Caveat: support only UTC time. Timezone support? ¯\_(ツ)_/¯
    // params:
    //   0000-2345, allow every 15 minutes interval. Get off at 17:48? ¯\_(ツ)_/¯
    //   comma seperated string of MON,TUE,WED,THU,FRI,SAT,SUN
    const _handleIncorrectTimeFormat = async () => this.settings.setOffFromWorkTimeUTCIncorrectFormat({
      chatId: message.chat.id,
      chatTitle: message.chat.title
    })
    if (text.match(/\/set[oO]ff[fF]rom[wW]ork[tT]imeUTC/)) {
      this.logger.info('newMessage setOffFromWorkTimeUTC')
      const params = text.split(' ')
      if (params.length === 3) {
        const time = params[1]
        const rawWorkday = params[2]
        if (time.match(/^([0-1][0-9]|2[0-3])(00|15|30|45)$/) &&
          rawWorkday.match(/^((MON|TUE|WED|THU|FRI|SAT|SUN),?){0,6}(MON|TUE|WED|THU|FRI|SAT|SUN)$/)) {
          // E.g. MON,MON will be considered as MON
          const workday = [...new Set(rawWorkday.split(','))].join(',')
          this.logger.info(`normalised - time: ${time}, workday: ${workday}`)
          await this.sqs.sendSetOffFromWorkTimeUTC({ message, time, workday })
        } else {
          this.logger.info(`time: ${time}, rawWorkday: ${rawWorkday}`)
          await _handleIncorrectTimeFormat()
        }
      } else {
        await _handleIncorrectTimeFormat()
      }
    }
  }

  async newMessage (event) {
    this.logger.info(`newMessage start at ${moment().utcOffset(8).format()}`)
    try {
      const xForwardedFor = event.headers['X-Forwarded-For'] || event.headers['x-forwarded-for']
      const requestIP = xForwardedFor.split(', ')[0]
      // should use WAF
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
      if (this.isBotCommand(message)) {
        await this.handleBotCommand(message)
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
