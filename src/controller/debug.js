require('babel-polyfill')

import async from 'async'
import _ from 'lodash'
import log from 'log-to-file-and-console-node'
import MessageController from './messageFacade'

class DebugController {

  constructor (bot) {
    this.bot = bot
  }

  async healthCheck (msg) {
    this.bot.sendMessage(msg.chat.id, 'debug mode start')
    const chatIds = await MessageController.getAllGroupIds()
    this.bot.sendMessage(msg.chat.id, 'getAllGroupIds, found: ' + chatIds.length)
    let groupCounter = 0
    let totalMessageCounter = 0
    const messageCountForGroupRegexp = /(?:^|\s)message: (.*?)(?:\s|$)/gm
    async.each(chatIds, async (chatId, callback) => {
      const msg = {chat: {id: chatId}}
      log.i('chatId: ' + JSON.stringify(msg))
      const message = await MessageController.getTopTen(msg, true)
      if (!_.isEmpty(message)) {
        log.i('message: \n\n' + message)
        groupCounter += 1
        try {
          const match = messageCountForGroupRegexp.exec(message)
          const totalMessage = Number(match[1])
          totalMessageCounter += totalMessage
          log.i('totalMessage: ' + totalMessage)
        } catch (e) {
          log.e('totalMessage error: ' + JSON.stringify(e))
        }
      }
      callback(null)
    }, err => {
      log.i('debug mode end')
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

export default DebugController
