require('babel-polyfill')

import 'moment'
import moment from 'moment-timezone'
import _ from 'lodash'
import log from 'log-to-file-and-console-node'
import mongoose from 'mongoose'
import UsageClass from '../model/usage'
import c from '../constants'
import SystemAdmin from '../helper/systemAdmin'

const systemAdmin = new SystemAdmin()
mongoose.Promise = global.Promise

export default class UsageController {

  constructor () {
    const connectionStringCache = process.env.MONGODB_CACHE_DO_URL
    const connectionStringPersistence = process.env.MONGODB_URL
    const cacheConnection = mongoose.createConnection(connectionStringCache)
    const persistenceConnection = mongoose.createConnection(connectionStringPersistence)
    this.UsageCache = cacheConnection.model('Usage', UsageClass.getSchema())
    this.UsagePersistence = persistenceConnection.model('Usage', UsageClass.getSchema())
  }

  addUsage (msg) {
    const usageCache = new this.UsageCache()
    usageCache.chatId = msg.chat.id || ''
    const usagePersistence = new this.UsagePersistence()
    usagePersistence.chatId = msg.chat.id || ''
    return Promise.all([
      usageCache.save(),
      usagePersistence.save()
    ])
  }

  updateUsageNotice (chatId) {
    return this.UsageCache.findOneAndUpdate(
      {chatId: chatId},
      {notified: true},
      {sort: '-dateCreated'})
      .exec()
  }

  async isAllowCommand (msg, force) {
    if (force || systemAdmin.isAdmin(msg)) { return }
    const chatId = msg.chat.id.toString()
    const usages = await this.UsageCache
      .find({chatId: chatId.toString()})
      .sort('-dateCreated')
      .limit(1)
      .exec()
    if (_.isArray(usages) && !_.isEmpty(usages)) {
      const usage = usages[0]
      const diff = Math.abs(moment(usage.dateCreated).diff(moment(), 'minute', true))
      if (diff < c.CONFIG.COMMAND_COOLDOWN_TIME) {
        if (!usage.notified) { await this.updateUsageNotice(chatId) }
        throw usage
      }
    }
  }

  async cleanup () {
    let shouldRepeat = true
    while (shouldRepeat) {
      const docs = await this.UsageCache
        .find({dateCreated: {$lt: new Date(moment().subtract(7, 'day').toISOString())}})
        .select('_id')
        .sort({_id: 1})
        .limit(c.CONFIG.CLEANUP_NUMBER_TO_DELETE)
        .exec()
      const ids = docs.map(doc => doc._id)
      const result = await this.UsageCache.remove({_id: {$in: ids}})
      const numberDeleted = result.result.n
      log.i(`cleanup usage cache database, numberDeleted: ${numberDeleted}`)
      shouldRepeat = (numberDeleted === c.CONFIG.CLEANUP_NUMBER_TO_DELETE)
    }
  }

}
