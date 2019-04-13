import DynamoDB from './dynamodb'
import Pino from 'pino'

export default class OffFromWork {
  constructor () {
    this.dynamodb = new DynamoDB()
    this.logger = new Pino({ level: process.env.LOG_LEVEL })
  }

  async off () {
    try {
      const rows = await this.dynamodb.getAllRowsWithinDays({ days: 7 })
      this.logger.debug('rows.length', rows.length)
      this.logger.debug('rows[0]', rows[0])
      // const statsMessage = await this.generateReport(rows, options)
      // await jung2botUtil.sendMessage(message.chat.id, statsMessage)
      // return statsMessage
      return true
    } catch (e) {
      this.logger.error(e.message)
      throw e
    }
  }
}
