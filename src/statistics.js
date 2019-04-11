import DynamoDB from './dynamodb'
import Jung2botUtil from './jung2botUtil'
import moment from 'moment'
import Pino from 'pino'

const jung2botUtil = new Jung2botUtil()

export default class Statistics {
  constructor (option) {
    this.dynamodb = new DynamoDB(option)
    this.logger = new Pino({ level: process.env.LOG_LEVEL })
  }

  async normaliseRows (rows) {
    const tally = rows.reduce((soFar, row) => {
      soFar[row.userId] = soFar[row.userId] ? soFar[row.userId] + 1 : 1
      return soFar
    }, {})
    const usersCount = rows.reduce((soFar, row) => {
      if (!soFar.check[row.userId]) {
        soFar.check[row.userId] = true
        soFar.users.push({
          userId: row.userId,
          firstName: row.firstName,
          lastName: row.lastName,
          fullName: [row.firstName, row.lastName].join(' '),
          dateCreated: row.dateCreated
        })
      }
      return soFar
    }, { users: [], check: {} })
    const rankings = usersCount.users.map(o => {
      o.count = tally[o.userId]
      return o
    }).sort((a, b) => {
      return a.count - b.count
    })
    return {
      totalMessage: rows.length,
      rankings
    }
  }

  async generateReport (rows, options = {}) {
    const normalisedRows = await this.normaliseRows(rows)
    const limit = options.limit || undefined

    const telegramMessageLimit = 3800
    let isReachingTelegramMessageLimit = false

    const header = `${limit ? 'Top ' + limit : 'All'} 冗員s in the last 7 days (last 上水 time):\n\n`

    let body = ''
    const loopLimit = limit ? Math.min(limit, normalisedRows.rankings.length) : normalisedRows.rankings.length
    for (let i = 0; i < loopLimit; i++) {
      if (body.length < telegramMessageLimit) {
        const o = normalisedRows.rankings[i]
        this.logger.debug('generateReport line 53, o:', o)
        const percentage = ((o.count / normalisedRows.totalMessage) * 100).toFixed(2)
        const timeAgo = moment(o.dateCreated).fromNow()
        const item = `${i + 1}. ${o.fullName} ${percentage}% (${timeAgo})\n`
        body += item
      } else {
        isReachingTelegramMessageLimit = true
        break
      }
    }
    body = isReachingTelegramMessageLimit ? `${body}...\n...\n` : body

    const footer = `\nTotal messages: ${normalisedRows.totalMessage}`

    const fullMessage = header + body + footer
    this.logger.debug('fullMessage', fullMessage)
    return fullMessage
  }

  async getStats (message, options) {
    const rows = await this.dynamodb.getRowsByChatId(message.chat.id)
    const statsMessage = await this.generateReport(rows, options)
    try {
      await jung2botUtil.sendMessage(message.chat.id, statsMessage)
      return statsMessage
    } catch (e) {
      this.logger.error(e.message)
      throw e
    }
  }

  async allJung (message) {
    return this.getStats(message)
  }

  async topTen (message) {
    return this.getStats(message, { limit: 10 })
  }
}
