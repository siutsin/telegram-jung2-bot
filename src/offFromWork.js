const moment = require('moment')
const Pino = require('pino')
const { DateTime } = require('luxon')

const DynamoDB = require('./dynamodb')

class OffFromWork {
  constructor () {
    this.dynamodb = new DynamoDB()
    this.logger = new Pino({ level: process.env.LOG_LEVEL })
  }

  async getOffChatIds (timeString) {
    this.logger.info(`getOffChatIds start at ${moment().format()}`)
    const cronTime = DateTime.fromISO(timeString)
    const offTime = cronTime.toFormat('HHmm')
    const weekday = cronTime.weekdayShort.toUpperCase()
    const rows = await this.dynamodb.getAllGroupIds({ offTime, weekday })
    return rows.map(o => o.chatId)
  }
}

module.exports = OffFromWork
