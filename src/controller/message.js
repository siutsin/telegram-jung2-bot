require('babel-polyfill')

import 'moment'
import moment from 'moment-timezone'
import log from 'log-to-file-and-console-node'
import mongoose from 'mongoose'
import MessageClass from '../model/message'
import UsageController from './usage'
import c from '../constants'

const usageController = new UsageController()
mongoose.Promise = global.Promise

export default class MessageController {

  constructor () {
    const connectionStringCacheDO = process.env.MONGODB_CACHE_DO_URL
    const connectionStringPersistence = process.env.MONGODB_URL
    const cacheDOConnection = mongoose.createConnection(connectionStringCacheDO)
    const persistenceConnection = mongoose.createConnection(connectionStringPersistence)
    this.MessageDOCache = cacheDOConnection.model('Message', MessageClass.getSchema())
    this.MessagePersistence = persistenceConnection.model('Message', MessageClass.getSchema())
    this.cachedLastSender = {}
  }

  async getCount (msg) {
    const chatId = msg.chat.id.toString()
    return this.MessageDOCache
      .count({
        chatId: chatId.toString(),
        dateCreated: {$gte: new Date(moment().subtract(7, 'day').toISOString())}
      }).exec()
  }

  async getJung (msg, limit) {
    const chatId = msg.chat.id.toString()
    let query = this.MessageDOCache.aggregate(
      [
        {
          $match: {
            chatId: chatId.toString(),
            dateCreated: {
              $gte: new Date(moment().subtract(7, 'day').toISOString())
            }
          }
        },
        {
          $group: {
            _id: '$userId',
            username: {$last: '$username'},
            firstName: {$last: '$firstName'},
            lastName: {$last: '$lastName'},
            dateCreated: {$last: '$dateCreated'},
            count: {$sum: 1}
          }
        }
      ])
      .sort('-count')
    if (limit > 0) {
      query = query.limit(limit)
    }
    return query.exec()
  }

  async getCountAndGetJung (msg, limit) {
    const results = await Promise.all([
      this.getCount(msg),
      this.getJung(msg, limit)
    ])
    const total = results[0]
    const getJungResults = results[1]
    for (const result of getJungResults) {
      result.total = total
      result.percent = ((result.count / total) * 100).toFixed(2) + '%'
    }
    return getJungResults
  }

  totalMessageForResults (message, results) {
    let total = ''
    let isOutOfLimit = false
    for (var i = 0, l = results.length; i < l; i++) {
      total = results[i].total
      if (message.length < c.MESSAGE.LIMIT) {
        message += `${(i + 1)}. ${results[i].firstName} ${results[i].lastName} ${results[i].percent} (${moment(results[i].dateCreated).fromNow()})\n`
      } else {
        isOutOfLimit = true
      }
    }
    message = isOutOfLimit ? `${message}...\n...\n` : message
    message += `\nTotal message: ${total}`
    return message
  }

  async getJungMessage (msg, limit, force) {
    let message = limit ? c.MESSAGE.TOP_TEN_TITLE : c.MESSAGE.ALL_JUNG_TITLE
    try {
      await usageController.isAllowCommand(msg, force)
      const results = await Promise.all([
        usageController.addUsage(msg),
        this.getCountAndGetJung(msg, limit)
      ])
      return this.totalMessageForResults(message, results[1])
    } catch (usage) {
      if (usage.notified) {
        message = ''
      } else {
        const oneMinutesLater = moment(usage.dateCreated)
          .add(c.CONFIG.COMMAND_COOLDOWN_TIME, 'minute')
          .tz(c.CONFIG.TIMEZONE)
        message = `[Error] Commands will be available ${oneMinutesLater.fromNow()}\n(${oneMinutesLater.format('h:mm:ss a')} HKT)`
      }
      return message
    }
  }

  shouldAddMessage (msg) {
    let result = true
    const chatId = msg.chat.id.toString()
    const userId = msg.from.id.toString()
    const isReplyingToMsg = !!msg.reply_to_message
    if (isReplyingToMsg || !this.cachedLastSender[chatId]) {
      this.cachedLastSender[chatId] = userId
    } else if (this.cachedLastSender[chatId] === userId) {
      result = false
    }
    return result
  }

  async saveToMessage (msg, messageObject) {
    messageObject.chatId = msg.chat.id || ''
    messageObject.chatTitle = msg.chat.title || ''
    messageObject.userId = msg.from.id || ''
    messageObject.username = msg.from.username || ''
    messageObject.firstName = msg.from.first_name || ''
    messageObject.lastName = msg.from.last_name || ''
    return messageObject.save()
  }

  async saveToMessageCacheDO (msg) {
    const msgCacheDO = new this.MessageDOCache()
    return this.saveToMessage(msg, msgCacheDO)
  }

  async saveToMessagePersistence (msg) {
    const msgPersistence = new this.MessagePersistence()
    return this.saveToMessage(msg, msgPersistence)
  }

  async addMessage (msg) {
    this.cachedLastSender[msg.chat.id] = msg.from.id.toString()
    return await Promise.all([
      this.saveToMessageCacheDO(msg),
      this.saveToMessagePersistence(msg)
    ])
  }

  async getAllGroupIds () {
    return this.MessageDOCache
      .find({dateCreated: {$gte: new Date(moment().subtract(7, 'day').toISOString())}})
      .distinct('chatId')
      .exec()
  }

  getAllJung (msg, force) {
    return this.getJungMessage(msg, 0, force)
  }

  getTopTen (msg, force) {
    return this.getJungMessage(msg, 10, force)
  }

  async cleanup () {
    let shouldRepeat = true
    while (shouldRepeat) {
      const docs = await this.MessageDOCache
        .find({dateCreated: {$lt: new Date(moment().subtract(7, 'day').toISOString())}})
        .select('_id')
        .sort({_id: 1})
        .limit(c.CONFIG.CLEANUP_NUMBER_TO_DELETE)
        .exec()
      const ids = docs.map(doc => doc._id)
      const result = await this.MessageDOCache.remove({_id: {$in: ids}})
      const numberDeleted = result.result.n
      log.i(`numberDeleted: ${numberDeleted}`)
      shouldRepeat = (numberDeleted === c.CONFIG.CLEANUP_NUMBER_TO_DELETE)
    }
  }

}
