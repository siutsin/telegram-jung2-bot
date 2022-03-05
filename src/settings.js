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

  async setOffFromWorkTimeUTCIncorrectFormat ({ chatId, chatTitle }) {
    this.logger.info(`setOffFromWorkTimeUTCIncorrectFormat start at ${moment().utcOffset(8).format()}`)
    await this.telegram.sendMessage(chatId, `
圍爐區: ${chatTitle}

Error: Invalid format for setOffFromWorkTimeUTC

Format:
/setOffFromWorkTimeUTC {{ 0000-2345, 15 minutes interval }} {{ MON,TUE,WED,THU,FRI,SAT,SUN }}
E.g.:
/setOffFromWorkTimeUTC 1800 MON,TUE,WED,THU,FRI
`)
  }

  async setOffFromWorkTimeUTC ({ chatId, chatTitle, userId, offTime, workday }) {
    this.logger.info(`setOffFromWorkTimeUTC start at ${moment().utcOffset(8).format()}`)
    if (await this.isAdmin({ chatId, userId })) {
      await this.dynamodb.setOffFromWorkTimeUTC({ chatId, chatTitle, userId, offTime, workday })
      await this.telegram.sendMessage(chatId, `
圍爐區: ${chatTitle}

Updated setOffFromWorkTime in UTC: ${offTime} ${workday}`)
    }
  }
}

module.exports = Settings
