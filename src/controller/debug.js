require('babel-polyfill')

import async from 'async'
import _ from 'lodash'
import log from 'log-to-file-and-console-node'
import MessageController from './messageFacade'

const totalNumberForMessage = message => {
  const messageCountForGroupRegexp = /(?:^|\s)message: (.*?)(?:\s|$)/gm
  const match = messageCountForGroupRegexp.exec(message)
  return Number(match[1])
}

export default class DebugController {

  constructor (bot) {
    this.bot = bot
  }

  async healthCheck (msg) {
    this.bot.sendMessage(msg.chat.id, 'debug mode start')
    const chatIds = await MessageController.getAllGroupIds()
    this.bot.sendMessage(msg.chat.id, 'getAllGroupIds, found: ' + chatIds.length)
    let groupCounter = 0
    let totalMessageCounter = 0
    async.each(chatIds, async (chatId, next) => {
      const msg = {chat: {id: chatId}}
      log.i('chatId: ' + JSON.stringify(msg), process.env.DISABLE_LOGGING)
      const message = await MessageController.getTopTen(msg, true)
      if (!_.isEmpty(message)) {
        log.i('message: \n\n' + message, process.env.DISABLE_LOGGING)
        groupCounter += 1
        try {
          const totalMessage = totalNumberForMessage(message)
          totalMessageCounter += totalMessage
          log.i('totalMessage: ' + totalMessage, process.env.DISABLE_LOGGING)
        } catch (e) {
          log.e('totalMessage error: ' + JSON.stringify(e), process.env.DISABLE_LOGGING)
        }
      }
      next()
    }, err => {
      log.i('debug mode end', process.env.DISABLE_LOGGING)
      if (!err) {
        this.bot.sendMessage(msg.chat.id,
          'debug mode end:\nget topTen for no. of groups: ' + groupCounter +
          '\ntotol no. of message in 7 days: ' + totalMessageCounter)
      } else {
        this.bot.sendMessage(msg.chat.id, err)
      }
    })
  }
}
