import Pino from 'pino'
import moment from 'moment'
import DynamoDB from './dynamodb'
import Help from './help'
import Statistics from './statistics'

export default class Messages {
  constructor () {
    this.dynamodb = new DynamoDB()
    this.logger = new Pino({ level: process.env.LOG_LEVEL })
    this.help = new Help()
    this.statistics = new Statistics()
  }

  async newMessage (event) {
    this.logger.info(`newMessage start at ${moment().utcOffset(8).format()}`)
    this.logger.debug(`event`, event)
    try {
      const params = JSON.parse(event.body)
      this.logger.trace('params', params)
      if (!params.message) {
        // handle edited_message
        return { statusCode: 204 }
      }
      const message = params.message
      await this.dynamodb.saveMessage({ message })
      if (message.entities &&
        message.entities[0] &&
        message.entities[0].type === 'bot_command') {
        const text = message.text
        this.logger.info(text)
        if (text.match(/\/jung[hH]elp/)) {
          this.logger.info(`newMessage help start at ${moment().utcOffset(8).format()}`)
          await this.help.sendHelpMessage(message)
          this.logger.info(`newMessage help finish at ${moment().utcOffset(8).format()}`)
        }
        if (text.match(/\/top[tT]en/)) {
          this.logger.info(`newMessage topTen start at ${moment().utcOffset(8).format()}`)
          await this.statistics.topTen(message)
          this.logger.info(`newMessage topTen finish at ${moment().utcOffset(8).format()}`)
        }
        if (text.match(/\/all[jJ]ung/)) {
          this.logger.info(`newMessage alljung start at ${moment().utcOffset(8).format()}`)
          await this.statistics.allJung(message)
          this.logger.info(`newMessage alljung finish at ${moment().utcOffset(8).format()}`)
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
