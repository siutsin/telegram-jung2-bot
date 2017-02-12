'use strict'

// This is just a message controller facade

var MessageController = require('./' + process.env.MESSAGE_CONTROLLER)

exports.init = function (skip) {
  if (MessageController.init && !skip) {
    MessageController.init()
  }
}

exports.clearCachedLastSender = MessageController.clearCachedLastSender

exports.setCachedLastSender = MessageController.setCachedLastSender

exports.shouldAddMessage = MessageController.shouldAddMessage

exports.addMessage = MessageController.addMessage

exports.getAllGroupIds = MessageController.getAllGroupIds

exports.getAllJung = MessageController.getAllJung

exports.getTopTen = MessageController.getTopTen

exports.cleanup = MessageController.cleanup
