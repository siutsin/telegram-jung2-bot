const Pino = require('pino')
const moment = require('moment')

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

  async handleBotCommand (message) {
    const text = message.text
    this.logger.info(text)
    if (text.match(/\/jungHelp/i)) {
      this.logger.info('newMessage jungHelp')
      await this.sqs.sendJungHelpMessage(message)
    }
    if (text.match(/\/topTen/i)) {
      this.logger.info('newMessage topTen')
      await this.sqs.sendTopTenMessage(message)
    }
    if (text.match(/\/topDiver/i)) {
      this.logger.info('newMessage topDiver')
      await this.sqs.sendTopDiverMessage(message)
    }
    if (text.match(/\/allJung/i)) {
      this.logger.info('newMessage allJung')
      await this.sqs.sendAllJungMessage(message)
    }
    if (text.match(/\/enableAllJung/i)) {
      this.logger.info('newMessage enableAllJung')
      await this.sqs.sendEnableAllJungMessage(message)
    }
    if (text.match(/\/disableAllJung/i)) {
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
    if (text.match(/\/setOffFromWorkTimeUTC/i)) {
      this.logger.info('newMessage setOffFromWorkTimeUTC')
      const params = text.split(' ')
      if (params.length === 3) {
        const offTime = params[1]
        const rawWorkday = params[2]
        if (offTime.match(/^([0-1]\d|2[0-3])(00|15|30|45)$/) &&
          rawWorkday.match(/^((MON|TUE|WED|THU|FRI|SAT|SUN),?){0,6}(MON|TUE|WED|THU|FRI|SAT|SUN)$/)) {
          // E.g. MON,MON will be considered as MON
          const workday = [...new Set(rawWorkday.split(','))].join(',')
          this.logger.info(`normalised - offTime: ${offTime}, workday: ${workday}`)
          await this.sqs.sendSetOffFromWorkTimeUTC({ message, offTime, workday })
        } else {
          this.logger.info(`offTime: ${offTime}, rawWorkday: ${rawWorkday}`)
          await _handleIncorrectTimeFormat()
        }
      } else {
        await _handleIncorrectTimeFormat()
      }
    }
  }

  async newMessage (event) {
    this.logger.info(`newMessage start at ${moment().format()}`)
    try {
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
      this.logger.info(`newMessage finish at ${moment().format()}`)
      return { statusCode: 200 }
    } catch (e) {
      this.logger.error(e.message)
      this.logger.info(`newMessage finish with error at ${moment().format()}`)
      return { statusCode: 500 }
    }
  }
}

module.exports = Messages
