import Pino from 'pino'
import DynamoDB from './dynamodb'
import Jung2botUtil from './jung2botUtil'
import Statistics from './statistics'
import pThrottle from 'p-throttle'

const jung2botUtil = new Jung2botUtil()

export default class OffFromWork {
  constructor () {
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

  async announcement (groupIds) {
    // https://core.telegram.org/bots/faq#my-bot-is-hitting-limits-how-do-i-avoid-this
    const GROUPS_PER_SECOND = 20
    const throttled = pThrottle(id => jung2botUtil.sendMessage(id, '夠鐘收工~~'), GROUPS_PER_SECOND, 1000)
    for (const id of groupIds) {
      await throttled(id)
    }
  }

  async statsPerGroup (groupIds, records) {
    const GROUPS_PER_SECOND = 20
    const throttled = pThrottle((id, report) => jung2botUtil.sendMessage(id, report), GROUPS_PER_SECOND, 1000)
    for (const id of groupIds) {
      const rawRowData = records[id]
      const report = await this.statistics.generateReport(rawRowData)
      await throttled(id, report)
    }
  }

  async off () {
    try {
      const rows = await this.dynamodb.getAllRowsWithinDays({ days: 7 })
      const records = await this.separateByGroups(rows)
      const groupIds = Object.keys(records)
      await this.announcement(groupIds)
      await this.statsPerGroup(groupIds, records)
      return true
    } catch (e) {
      this.logger.error(e.message)
      throw e
    }
  }
}
