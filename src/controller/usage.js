'use strict'

import mongoose from 'mongoose'
import UsageClass from '../model/usage'
var UsageCache
var UsagePersistence

var Constants = require('../constants')
require('moment')
var moment = require('moment-timezone')
var _ = require('lodash')
var log = require('log-to-file-and-console-node')
var async = require('async')
import SystemAdmin from '../helper/systemAdmin'
const systemAdmin = new SystemAdmin()

// TODO: refactoring required
exports.init = function () {
  var connectionStringCache = '127.0.0.1:27017/jung2botCache'
  if (process.env.OPENSHIFT_MONGODB_DB_PASSWORD) {
    connectionStringCache = process.env.OPENSHIFT_MONGODB_DB_USERNAME + ':' +
      process.env.OPENSHIFT_MONGODB_DB_PASSWORD + '@' +
      process.env.OPENSHIFT_MONGODB_DB_HOST + ':' +
      process.env.OPENSHIFT_MONGODB_DB_PORT + '/' +
      process.env.OPENSHIFT_APP_NAME
  }

  var connectionStringPersistence = '127.0.0.1:27017/jung2bot'
  if (process.env.MONGODB_URL) {
    connectionStringPersistence = process.env.MONGODB_URL
  }

  var cacheConnection = mongoose.createConnection(connectionStringCache)
  var persistenceConnection = mongoose.createConnection(connectionStringPersistence)
  UsageCache = cacheConnection.model('Usage', UsageClass.getSchema())
  UsagePersistence = persistenceConnection.model('Usage', UsageClass.getSchema())
}

exports.addUsage = function (msg) {
  var usageCache = new UsageCache()
  usageCache.chatId = msg.chat.id || ''
  var usagePersistence = new UsagePersistence()
  usagePersistence.chatId = msg.chat.id || ''
  var promises = [
    usageCache.save(),
    usagePersistence.save()
  ]
  return Promise.all(promises)
}

var updateUsageNotice = function (chatId) {
  var promise = new mongoose.Promise()
  UsageCache.findOneAndUpdate(
    {chatId: chatId},
    {notified: true},
    {sort: '-dateCreated'},
    function callback (err, foundUsage) {
      if (err) { throw err }
      promise.complete(foundUsage)
    }
  )
  return promise
}

exports.isAllowCommand = function (msg, force) {
  var promise = new mongoose.Promise()
  if (force || systemAdmin.isAdmin(msg)) {
    return promise.complete()
  }
  var chatId = msg.chat.id.toString()
  UsageCache.find({chatId: chatId.toString()})
    .sort('-dateCreated')
    .limit(1)
    .exec(function (err, usages) {
      if (err) { throw err }
      if (_.isArray(usages) && !_.isEmpty(usages)) {
        var usage = usages[0]
        var diff = Math.abs(moment(usage.dateCreated).diff(moment(), 'minute', true))
        if (diff < Constants.CONFIG.COMMAND_COOLDOWN_TIME) {
          if (usage.notified) {
            promise.reject(usage)
          } else {
            updateUsageNotice(chatId).then(function () {
              promise.reject(usage)
            })
          }
        } else {
          promise.complete()
        }
      } else {
        promise.complete()
      }
    })
  return promise
}

exports.cleanup = function () {
  const numberToDelete = 10000
  var shouldRepeat = true
  var promise = new mongoose.Promise()
  async.whilst(
    function test () {
      return shouldRepeat
    },
    function iteratee (next) {
      UsageCache.find({
        dateCreated: {
          $lt: new Date(moment().subtract(7, 'day').toISOString())
        }
      }).select('_id')
        .sort({_id: 1})
        .limit(numberToDelete)
        .exec(function (err, docs) {
          if (err) { throw err }
          var ids = docs.map(function (doc) {
            return doc._id
          })
          UsageCache.remove({_id: {$in: ids}}, function (err, result) {
            if (err) {
              next(err)
            } else {
              var numberDeleted = result.result.n
              log.i('cleanup usage cache database, numberDeleted: ' + numberDeleted)
              shouldRepeat = (numberDeleted === numberToDelete)
              next()
            }
          })
        }
        )
    },
    function callback (err) {
      if (err) {
        log.e(err)
        promise.error(err)
      } else {
        promise.complete()
      }
    })
  return promise
}
