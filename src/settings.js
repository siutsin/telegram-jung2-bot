import Pino from 'pino'
import moment from 'moment'
import Telegram from './telegram'
import DynamoDB from './dynamodb'

export default class Settings {
  constructor () {
    this.telegram = new Telegram()
    this.dynamodb = new DynamoDB()
    this.logger = new Pino({ level: process.env.LOG_LEVEL })
  }

  async isAdmin ({ chatId, userId, allAdmin }) {
    this.logger.info(`isAdmin start at ${moment().utcOffset(8).format()}`)
    let isAdmin = allAdmin
    if (!isAdmin) {
      isAdmin = await this.telegram.isAdmin({ chatId, userId })
    }
    return isAdmin
  }

  async isAllJungEnabled ({ chatId }) {
    this.logger.info(`isAllJungEnabled start at ${moment().utcOffset(8).format()}`)
    const response = await this.dynamodb.getStatsByChatId({ chatId })
    const isAllJungEnabled = response.Items[0].enableAllJung
    this.logger.info('isAllJungEnabled', isAllJungEnabled)
    return !!isAllJungEnabled
  }

  async enableAllJung ({ chatId, chatTitle, userId, allAdmin }) {
    this.logger.info(`enableAllJung start at ${moment().utcOffset(8).format()}`)
    if (await this.isAdmin({ chatId, userId, allAdmin })) {
      await this.dynamodb.enableAllJung({ chatId })
      await this.telegram.sendMessage(chatId, `圍爐區: ${chatTitle} - Enabled AllJung command`)
    }
  }

  async disableAllJung ({ chatId, chatTitle, userId, allAdmin }) {
    this.logger.info(`disableAllJung start at ${moment().utcOffset(8).format()}`)
    if (await this.isAdmin({ chatId, userId, allAdmin })) {
      await this.dynamodb.disableAllJung({ chatId })
      await this.telegram.sendMessage(chatId, `圍爐區: ${chatTitle} - Disabled AllJung command`)
    }
  }
}
