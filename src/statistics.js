import DynamoDB from './dynamodb'
import Jung2botUtil from './jung2botUtil'
import moment from 'moment'
import Pino from 'pino'

export default class Statistics {
  constructor () {
    this.jung2botUtil = new Jung2botUtil()
    this.dynamodb = new DynamoDB()
    this.logger = new Pino({ level: process.env.LOG_LEVEL })
  }

  async normaliseRows (rows, options) {
    this.logger.info(`normaliseRows start at ${moment().utcOffset(8).format()}`)
    const reverse = options.reverse || undefined
    const tally = rows.reduce((soFar, row) => {
      soFar[row.userId] = soFar[row.userId] ? soFar[row.userId] + 1 : 1
      return soFar
    }, {})
    const usersCount = rows
      .reduce((soFar, row) => {
        if (!soFar.check[row.userId]) {
          soFar.check[row.userId] = true
          soFar.users.push({
            userId: row.userId,
            chatTitle: row.chatTitle,
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
    })
    rankings.sort((a, b) => reverse ? a.count - b.count : b.count - a.count)
    this.logger.info(`normaliseRows finish at ${moment().utcOffset(8).format()}`)
    return {
      totalMessage: rows.length,
      rankings
    }
  }

  buildHeader ({ limit, reverse, chatTitle }) {
    let header = `圍爐區: ${chatTitle}`
    header += `\n\n`
    header += `${limit ? 'Top ' + limit : 'All'} `
    header += `${reverse ? '潛水員s' : '冗員s'} `
    header += `in the last 7 days (last 上水 time):`
    header += `\n\n`
    return header
  }

  buildBody ({ limit, normalisedRows }) {
    // Maximum length for a message is 4096 UTF8 characters
    // https://core.telegram.org/method/messages.sendMessage
    const telegramMessageLimit = 3800
    let isReachingTelegramMessageLimit = false
    let body = ''
    const loopLimit = limit ? Math.min(limit, normalisedRows.rankings.length) : normalisedRows.rankings.length
    for (let i = 0; i < loopLimit; i++) {
      if (body.length < telegramMessageLimit) {
        const o = normalisedRows.rankings[i]
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
    return body
  }

  buildFooter ({ normalisedRows, reverse }) {
    let footer = `\nTotal messages: ${normalisedRows.totalMessage}`
    footer += `\n\n`
    if (reverse) {
      footer += `between, 深潛會搵唔到 ho chi is`
      footer += `\n`
    }
    footer += `Last Update: ${moment().utcOffset(8).format()}`
    return footer
  }

  async generateReport (rows, options) {
    this.logger.info(`generateReport start at ${moment().utcOffset(8).format()}`)
    const normalisedRows = await this.normaliseRows(rows, options)
    this.logger.debug('normalisedRows.rankings', normalisedRows.rankings)
    options.normalisedRows = normalisedRows
    options.chatTitle = normalisedRows.rankings[0].chatTitle
    const header = this.buildHeader(options)
    const body = this.buildBody(options)
    const footer = this.buildFooter(options)
    const fullMessage = header + body + footer
    this.logger.trace('fullMessage', fullMessage)
    this.logger.info(`generateReport finish at ${moment().utcOffset(8).format()}`)
    return { fullMessage, userCount: normalisedRows.rankings.length, messageCount: normalisedRows.totalMessage }
  }

  async generateReportByChatId (chatId, options) {
    this.logger.info(`generateReportByChatId start at ${moment().utcOffset(8).format()}`)
    const rows = await this.dynamodb.getRowsByChatId({ chatId })
    const { fullMessage, userCount, messageCount } = await this.generateReport(rows, options)
    await this.dynamodb.updateChatIdMessagesCount({ chatId, userCount, messageCount })
    this.logger.info(`generateReportByChatId finish at ${moment().utcOffset(8).format()}`)
    return fullMessage
  }

  async getStats (chatId, options) {
    this.logger.info(`getStats start at ${moment().utcOffset(8).format()}`)
    let returnMessage = ''
    if (options.offFromWork) {
      returnMessage = '夠鐘收工~~\n\n'
    }
    try {
      const statsMessage = await this.generateReportByChatId(chatId, options)
      returnMessage += statsMessage
      this.logger.info(`got stats report, sending to telegram at ${moment().utcOffset(8).format()}`)
      await this.jung2botUtil.sendMessage(chatId, returnMessage)
    } catch (e) {
      this.logger.error(e.message)
      if (!e.message.match(/[45][0-9]{2}/)) { throw e }
      returnMessage = `bot is removed in group ${chatId}`
    }
    this.logger.info(`getStats finish at ${moment().utcOffset(8).format()}`)
    return returnMessage
  }

  async allJung (chatId) {
    return this.getStats(chatId, {})
  }

  async topTen (chatId) {
    return this.getStats(chatId, { limit: 10 })
  }

  async topDiver (chatId) {
    return this.getStats(chatId, { limit: 10, reverse: true })
  }

  async offFromWork (chatId) {
    return this.getStats(chatId, { limit: 10, offFromWork: true })
  }
}
