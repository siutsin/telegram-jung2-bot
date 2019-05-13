import Pino from 'pino'
import moment from 'moment'
import DynamoDB from './dynamodb'
import Jung2botUtil from './jung2botUtil'
import SQS from './sqs'

export default class OffFromWork {
  constructor () {
    this.jung2botUtil = new Jung2botUtil()
    this.dynamodb = new DynamoDB()
    this.sqs = new SQS()
    this.logger = new Pino({ level: process.env.LOG_LEVEL })
  }

  async statsPerGroup (chatIds) {
    this.logger.info(`statsPerGroup start at ${moment().utcOffset(8).format()}`)
    this.logger.debug('chatIds:', chatIds)
    const MAX_CONCURRENT = 100
    let promiseArray = []
    for (const chatId of chatIds) {
      this.logger.info(`chatId: ${chatId}`)
      if (promiseArray.length < MAX_CONCURRENT) {
        promiseArray.push(this.sqs.sendOffFromWorkMessage(chatId))
      }
      if (promiseArray.length >= MAX_CONCURRENT) {
        await Promise.all(promiseArray)
        promiseArray.length = 0
      }
    }
    if (promiseArray.length > 0) {
      await Promise.all(promiseArray)
    }
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
