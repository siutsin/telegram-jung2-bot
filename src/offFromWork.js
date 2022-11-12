const Pino = require('pino')
const { DateTime } = require('luxon')

const DynamoDB = require('./dynamodb')

class OffFromWork {
  constructor () {
    this.dynamodb = new DynamoDB()
    this.logger = new Pino({ level: process.env.LOG_LEVEL })
  }

  async getOffChatIds (timeString) {
    this.logger.info(`getOffChatIds start at ${DateTime.now().toISO()}`)
    const cronTime = DateTime.fromISO(timeString)
    const offTime = cronTime.toFormat('HHmm')
    const weekday = cronTime.weekdayShort.toUpperCase()
    const rows = await this.dynamodb.getAllGroupIds({ offTime, weekday })
    this.logger.info(`getOffChatIds finish at ${DateTime.now().toISO()}`)
    return rows.map(o => o.chatId)
  }
}

module.exports = OffFromWork
