import DynamoDB from './dynamodb'
import Settings from './settings'
import Telegram from './telegram'
import moment from 'moment'
import Pino from 'pino'

export default class Statistics {
  constructor () {
    this.telegram = new Telegram()
    this.dynamodb = new DynamoDB()
    this.settings = new Settings()
    this.logger = new Pino({ level: process.env.LOG_LEVEL })
  }

  async normaliseRows (options) {
    this.logger.info(`normaliseRows start at ${moment().utcOffset(8).format()}`)
    const rows = options.rows
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
    this.logger.info(`buildHeader start at ${moment().utcOffset(8).format()}`)
    let header = `圍爐區: ${chatTitle}`
    header += `\n\n`
    header += `${limit ? 'Top ' + limit : 'All'} `
    header += `${reverse ? '潛水員s' : '冗員s'} `
    header += `in the last 7 days`
    header += `${reverse ? ':' : ' (last 上水 time):'}`
    header += `\n\n`
    this.logger.info(`buildHeader finish at ${moment().utcOffset(8).format()}`)
    return header
  }

  buildBodyForDiver ({ limit, normalisedRows }) {
    this.logger.info(`buildBodyForDiver start at ${moment().utcOffset(8).format()}`)
    const cloneNormalisedRows = JSON.parse(JSON.stringify(normalisedRows))
    cloneNormalisedRows.rankings.sort((a, b) => moment(a.dateCreated).isBefore(moment(b.dateCreated)) ? -1 : 1)
    // TODO: limit is always 10 for now
    if (cloneNormalisedRows.rankings.length < limit) {
      limit = cloneNormalisedRows.rankings.length
    }
    let diverBody = ''
    for (let i = 0; i < limit; i++) {
      const o = cloneNormalisedRows.rankings[i]
      const timeAgo = moment(o.dateCreated).fromNow()
      const item = `${i + 1}. ${o.fullName} - ${timeAgo}\n`
      diverBody += item
    }
    this.logger.info(`buildBodyForDiver finish at ${moment().utcOffset(8).format()}`)
    return diverBody
  }

  buildBody ({ limit, reverse, normalisedRows }) {
    this.logger.info(`buildBody start at ${moment().utcOffset(8).format()}`)
    // Maximum length for a message is 4096 UTF8 characters
    // https://core.telegram.org/method/messages.sendMessage
    const telegramMessageLimit = 3800
    let isReachingTelegramMessageLimit = false
    let body = ''
    if (reverse) {
      body += 'By 冗power:'
      body += '\n'
    }
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
    if (isReachingTelegramMessageLimit) {
      body = `${body}...\n...\n`
    } else if (reverse) {
      body += '\n'
      body += 'By last 上水:'
      body += '\n'
      body += this.buildBodyForDiver({ limit, normalisedRows })
    }
    this.logger.info(`buildBody finish at ${moment().utcOffset(8).format()}`)
    return body
  }

  buildFooter ({ normalisedRows, reverse }) {
    this.logger.info(`buildFooter start at ${moment().utcOffset(8).format()}`)
    let footer = `\nTotal messages: ${normalisedRows.totalMessage}`
    footer += `\n\n`
    if (reverse) {
      footer += `between, 深潛會搵唔到 ho chi is`
      footer += `\n`
    }
    footer += `Last Update: ${moment().utcOffset(8).format()}`
    this.logger.info(`buildFooter finish at ${moment().utcOffset(8).format()}`)
    return footer
  }

  async generateReport (options) {
    this.logger.info(`generateReport start at ${moment().utcOffset(8).format()}`)
    const normalisedRows = await this.normaliseRows(options)
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

  async generateReportByChatId (options) {
    this.logger.info(`generateReportByChatId start at ${moment().utcOffset(8).format()}`)
    const chatId = options.chatId
    options.rows = await this.dynamodb.getRowsByChatId({ chatId })
    const { fullMessage, userCount, messageCount } = await this.generateReport(options)
    await this.dynamodb.updateChatIdMessagesCount({ chatId, userCount, messageCount })
    this.logger.info(`generateReportByChatId finish at ${moment().utcOffset(8).format()}`)
    return fullMessage
  }

  async getStats (options) {
    const chatId = options.chatId
    this.logger.info(`getStats start at ${moment().utcOffset(8).format()}`)
    let returnMessage = ''
    if (options.offFromWork) {
      returnMessage = '夠鐘收工~~\n\n'
    }
    try {
      const statsMessage = await this.generateReportByChatId(options)
      returnMessage += statsMessage
      this.logger.info(`got stats report, sending to telegram at ${moment().utcOffset(8).format()}`)
      await this.telegram.sendMessage(chatId, returnMessage)
    } catch (e) {
      this.logger.error(e.message)
      if (!e.message.match(/[45][0-9]{2}/)) { throw e }
      returnMessage = `bot is removed in group ${chatId}`
    }
    this.logger.info(`getStats finish at ${moment().utcOffset(8).format()}`)
    return returnMessage
  }

  async allJung ({ chatId }) {
    const isAllJungEnable = await this.settings.isAllJungEnabled({ chatId })
    if (isAllJungEnable) {
      return this.getStats({ chatId })
    }
    return false
  }

  async topTen ({ chatId }) {
    return this.getStats({ chatId, limit: 10 })
  }

  async topDiver ({ chatId }) {
    return this.getStats({ chatId, limit: 10, reverse: true })
  }

  async offFromWork ({ chatId }) {
    return this.getStats({ chatId, limit: 10, offFromWork: true })
  }
}
