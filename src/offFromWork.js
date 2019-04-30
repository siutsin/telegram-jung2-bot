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

  async separateByGroups (rows) {
    this.logger.info(`separateByGroups start at ${moment().utcOffset(8).format()}`)
    const records = rows.reduce((soFar, row) => {
      if (!soFar[row.chatId]) {
        soFar[row.chatId] = []
      }
      soFar[row.chatId].push(row)
      return soFar
    }, {})
    this.logger.trace('records', records)
    this.logger.info(`separateByGroups finish at ${moment().utcOffset(8).format()}`)
    return records
  }

  async statsPerGroup (groupIds, records) {
    // https://core.telegram.org/bots/faq#my-bot-is-hitting-limits-how-do-i-avoid-this
    // If you're sending bulk notifications to multiple users, the API will not allow more than 30 messages
    // per second or so. Consider spreading out notifications over large intervals of 8—12 hours for best results.
    // Also note that your bot will not be able to send more than 20 messages per minute to the same group.
    this.logger.info(`statsPerGroup start at ${moment().utcOffset(8).format()}`)
    const limiter = new Bottleneck({ // 25 messages per second
      maxConcurrent: 1,
      minTime: 40
    })
    this.logger.debug('groupIds:', groupIds)
    for (const id of groupIds) {
      const rawRowData = records[id]
      this.logger.info(`id: ${id} length: ${rawRowData.length}`)
      let report = await this.statistics.generateReport(rawRowData, { limit: 10 })
      report = '夠鐘收工~~\n\n' + report
      try {
        await limiter.schedule(() => this.jung2botUtil.sendMessage(id, report))
      } catch (e) {
        this.logger.error(`statsPerGroup error - id: ${id}, error: ${e.message}`)
        // allow only 4xx and 5xx telegram API error
        if (!e.message.match(/[45][0-9]{2}/)) { throw e }
      }
    }
    this.logger.info(`statsPerGroup finish at ${moment().utcOffset(8).format()}`)
  }

  async off () {
    this.logger.info(`off start at ${moment().utcOffset(8).format()}`)
    try {
      const rows = await this.dynamodb.getAllRowsWithinDays({ days: 7 })
      const records = await this.separateByGroups(rows)
      // Message ordered by the most active groups
      const orderedGroupIds = Object.keys(records)
        .map(chatId => ({ chatId: chatId, count: records[chatId].length }))
        .sort((a, b) => b.count - a.count)
        .map(o => o.chatId)
      this.logger.debug('orderedGroupIds.length', orderedGroupIds.length)
      // await this.statsPerGroup(orderedGroupIds, records)
      this.logger.info(`off finish at ${moment().utcOffset(8).format()}`)
      return true
    } catch (e) {
      this.logger.error(e.message)
      this.logger.info(`off finish with error at ${moment().utcOffset(8).format()}`)
      throw e
    }
  }
}
