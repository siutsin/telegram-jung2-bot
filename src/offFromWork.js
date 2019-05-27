import Pino from 'pino'
import moment from 'moment'
import DynamoDB from './dynamodb'
import SQS from './sqs'

export default class OffFromWork {
  constructor () {
    this.dynamodb = new DynamoDB()
    this.sqs = new SQS()
    this.logger = new Pino({ level: process.env.LOG_LEVEL })
  }

  async statsPerGroup (chatIds) {
    this.logger.info(`statsPerGroup start at ${moment().utcOffset(8).format()}`)
    this.logger.debug('chatIds:', chatIds)
    let promises = []
    for (const chatId of chatIds) {
      this.logger.info(`chatId: ${chatId}`)
      promises.push(this.sqs.sendOffFromWorkMessage(chatId))
    }
    await Promise.all(promises)
    this.logger.info(`statsPerGroup finish at ${moment().utcOffset(8).format()}`)
  }

  async off () {
    this.logger.info(`off start at ${moment().utcOffset(8).format()}`)
    const rows = await this.dynamodb.getAllGroupIds()
    const chatIds = rows.map(o => o.chatId)
    await this.statsPerGroup(chatIds)
    this.logger.info(`off finish at ${moment().utcOffset(8).format()}`)
    return true
  }
}
