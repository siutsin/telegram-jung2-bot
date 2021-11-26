const moment = require('moment')
const Pino = require('pino')
const Telegram = require('./telegram')
const DynamoDB = require('./dynamodb')

class Settings {
  constructor () {
    this.telegram = new Telegram()
    this.dynamodb = new DynamoDB()
    this.logger = new Pino({ level: process.env.LOG_LEVEL })
  }

  async isAdmin ({ chatId, userId }) {
    this.logger.info(`isAdmin start at ${moment().utcOffset(8).format()}`)
    return this.telegram.isAdmin({ chatId, userId })
  }

  async isAllJungEnabled ({ chatId }) {
    this.logger.info(`isAllJungEnabled start at ${moment().utcOffset(8).format()}`)
    const response = await this.dynamodb.getStatsByChatId({ chatId })
    let isAllJungEnabled = response.Items[0].enableAllJung
    if (isAllJungEnabled === undefined) {
      this.logger.info('isAllJungEnabled no record in settings, set to true')
      isAllJungEnabled = true
    }
    this.logger.info('isAllJungEnabled', isAllJungEnabled)
    return isAllJungEnabled
  }

  async enableAllJung ({ chatId, chatTitle, userId }) {
    this.logger.info(`enableAllJung start at ${moment().utcOffset(8).format()}`)
    if (await this.isAdmin({ chatId, userId })) {
      await this.dynamodb.enableAllJung({ chatId })
      await this.telegram.sendMessage(chatId, `
圍爐區: ${chatTitle}

Enabled AllJung command`)
    }
  }

  async disableAllJung ({ chatId, chatTitle, userId }) {
    this.logger.info(`disableAllJung start at ${moment().utcOffset(8).format()}`)
    if (await this.isAdmin({ chatId, userId })) {
      await this.dynamodb.disableAllJung({ chatId })
      await this.telegram.sendMessage(chatId, `
圍爐區: ${chatTitle}

Disabled AllJung command`)
    }
  }
}

module.exports = Settings
