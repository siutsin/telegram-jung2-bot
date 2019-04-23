import Pino from 'pino'
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
    const records = rows.reduce((soFar, row) => {
      if (!soFar[row.chatId]) {
        soFar[row.chatId] = []
      }
      soFar[row.chatId].push(row)
      return soFar
    }, {})
    this.logger.trace('records', records)
    return records
  }

  async statsPerGroup (groupIds, records) {
    // https://core.telegram.org/bots/faq#my-bot-is-hitting-limits-how-do-i-avoid-this
    // If you're sending bulk notifications to multiple users, the API will not allow more than 30 messages
    // per second or so. Consider spreading out notifications over large intervals of 8—12 hours for best results.
    // Also note that your bot will not be able to send more than 20 messages per minute to the same group.
    const GROUPS_PER_MINUTE = 19 // testing
    const PER_SECOND = 1000
    const PER_MINUTE = 60 * PER_SECOND
    const limiter = new Bottleneck({
      reservoir: GROUPS_PER_MINUTE,
      reservoirRefreshAmount: GROUPS_PER_MINUTE,
      reservoirRefreshInterval: PER_MINUTE,
      maxConcurrent: 10,
      minTime: PER_SECOND
    })
    this.logger.debug('groupIds:', groupIds)
    for (const id of groupIds) {
      const rawRowData = records[id]
      let report = await this.statistics.generateReport(rawRowData, { limit: 10 })
      report = '夠鐘收工~~\n\n' + report
      await limiter.schedule(() => this.jung2botUtil.sendMessage(id, report))
    }
  }

  async off () {
    try {
      const rows = await this.dynamodb.getAllRowsWithinDays({ days: 7 })
      const records = await this.separateByGroups(rows)
      // Message ordered by the most active groups
      const orderedGroupIds = Object.keys(records)
        .map(chatId => ({ chatId: chatId, count: records[chatId].length }))
        .sort((a, b) => b.count - a.count)
        .map(o => o.chatId)
      this.logger.debug('orderedGroupIds', orderedGroupIds)
      await this.statsPerGroup(orderedGroupIds, records)
      return true
    } catch (e) {
      this.logger.error(e.message)
      throw e
    }
  }
}
