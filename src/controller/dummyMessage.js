'use strict'

var log = require('log-to-file-and-console-node')
var Constants = require('../model/constants')
// var Message = require('./message')
var BaseMessage = require('./baseMessage')
require('moment')
var moment = require('moment-timezone')

// var cachedLastSender = BaseMessage.cachedLastSender

exports.clearCachedLastSender = BaseMessage.clearCachedLastSender

exports.setCachedLastSender = BaseMessage.setCachedLastSender

exports.shouldAddMessage = BaseMessage.shouldAddMessage

exports.addMessage = function (msg, callback) {
  var message = BaseMessage.createMessage(msg, callback)
  log.i('dummy.addMessage: ' + message, process.env.DISABLE_LOGGING)
}

exports.getAllGroupIds = function () {
  return Promise.resolve([0])
}

var getCount = function (msg) {
  return 100
}

var getJung = function (msg, limit) {
  return [{'firstName': 'First', 'lastName': 'Last', 'count': 50, 'dateCreated': new Date().getTime()}]
}

var getCountAndGetJung = function (msg, limit) {
  var total = getCount()
  var getJungResults = getJung()
  for (var i = 0, l = getJungResults.length; i < l; i++) {
    getJungResults[i].total = total
    getJungResults[i].percent = ((getJungResults[i].count / total) * 100).toFixed(2) + '%'
  }
  return getJungResults
}

var getJungMessage = function (msg, limit, force) {
  var message = limit ? Constants.MESSAGE.TOP_TEN_TITLE : Constants.MESSAGE.ALL_JUNG_TITLE
  var results = getCountAndGetJung(msg, limit)
  var total = ''
  for (var i = 0, l = results.length; i < l; i++) {
    total = results[i].total
    message += (i + 1) + '. ' + results[i].firstName + ' ' + results[i].lastName + ' ' + results[i].percent +
      ' (' + moment(results[i].dateCreated).fromNow() + ')' + '\n'
  }
  message += '\nTotal message: ' + total
  return Promise.resolve(message)
}
exports.getJungMessage = getJungMessage

exports.getAllJung = function (msg, force) {
  return getJungMessage(msg, 0, force)
}

exports.getTopTen = function (msg, force) {
  return getJungMessage(msg, 10, force)
}
