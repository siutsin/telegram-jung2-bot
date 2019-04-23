import Pino from 'pino'
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
    const promises = []
    for (const id of groupIds) {
      const rawRowData = records[id]
      let report = await this.statistics.generateReport(rawRowData, { limit: 10 })
      report = '夠鐘收工~~\n\n' + report
      promises.push(this.jung2botUtil.sendMessage(id, report))
    }
    await Promise.all(promises)
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
