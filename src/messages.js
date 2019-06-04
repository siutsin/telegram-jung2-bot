import Pino from 'pino'
import moment from 'moment'
import DynamoDB from './dynamodb'
import SQS from './sqs'

function _isBotCommand (message) {
  return message.entities && message.entities[0] && message.entities[0].type === 'bot_command'
}

export default class Messages {
  constructor () {
    this.dynamodb = new DynamoDB()
    this.logger = new Pino({ level: process.env.LOG_LEVEL })
    this.sqs = new SQS()
  }

  async newMessage (event) {
    this.logger.info(`newMessage start at ${moment().utcOffset(8).format()}`)
    try {
      const params = JSON.parse(event.body)
      this.logger.trace('params', params)
      const message = params.message
      if (!message || !message.chat.type.includes('group')) {
        // handle edited_message and non group
        return { statusCode: 204 }
      }
      await this.dynamodb.saveMessage({ message })
      if (_isBotCommand(message)) {
        const text = message.text
        this.logger.info(text)
        if (text.match(/\/jung[hH]elp/)) {
          this.logger.info(`newMessage help`)
          await this.sqs.sendJungHelpMessage(message)
        }
        if (text.match(/\/top[tT]en/)) {
          this.logger.info(`newMessage topTen`)
          await this.sqs.sendTopTenMessage(message)
        }
        if (text.match(/\/top[dD]iver/)) {
          this.logger.info(`newMessage topDiver`)
          await this.sqs.sendTopDiverMessage(message)
        }
        if (text.match(/\/all[jJ]ung/)) {
          this.logger.info(`newMessage alljung`)
          await this.sqs.sendAllJungMessage(message)
        }
        if (text.match(/\/enable[aA]ll[jJ]ung/)) {
          this.logger.info(`newMessage enableAllJung`)
          await this.sqs.sendEnableAllJungMessage(message)
        }
        if (text.match(/\/disable[aA]ll[jJ]ung/)) {
          this.logger.info(`newMessage disableAllJung`)
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
