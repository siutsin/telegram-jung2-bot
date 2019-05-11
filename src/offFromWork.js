import Pino from 'pino'
import moment from 'moment'
import Bottleneck from 'bottleneck'
import DynamoDB from './dynamodb'
import Jung2botUtil from './jung2botUtil'
import Statistics from './statistics'

export default class OffFromWork {
  constructor () {
    this.jung2botUtil = new Jung2botUtil()
    this.dynamodb = new DynamoDB()
    this.statistics = new Statistics()
    this.logger = new Pino({ level: process.env.LOG_LEVEL })
  }

  async statsPerGroup (chatIds) {
    this.logger.info(`statsPerGroup start at ${moment().utcOffset(8).format()}`)
    const limiter = new Bottleneck({ // 25 messages per second
      maxConcurrent: 1,
      minTime: 40
    })
    this.logger.debug('chatIds:', chatIds)
    for (const chatId of chatIds) {
      this.logger.info(`chatId: ${chatId}`)
      let report = await this.statistics.generateReportByChatId(chatId, { limit: 10 })
      report = '夠鐘收工~~\n\n' + report
      try {
        await limiter.schedule(() => this.jung2botUtil.sendMessage(chatId, report))
      } catch (e) {
        this.logger.error(`statsPerGroup error - id: ${chatId}, error: ${e.message}`)
        // allow only 4xx and 5xx telegram API error
        if (!e.message.match(/[45][0-9]{2}/)) { throw e }
      }
    }
    this.logger.info(`statsPerGroup finish at ${moment().utcOffset(8).format()}`)
  }

  async off () {
    this.logger.info(`off start at ${moment().utcOffset(8).format()}`)
    try {
      const rows = await this.dynamodb.getAllGroupIds()
      const chatIds = rows.map(o => o.chatId)
      await this.statsPerGroup(chatIds)
      this.logger.info(`off finish at ${moment().utcOffset(8).format()}`)
      return true
    } catch (e) {
      this.logger.error(e.message)
      this.logger.info(`off finish with error at ${moment().utcOffset(8).format()}`)
      throw e
    }
  }
}
