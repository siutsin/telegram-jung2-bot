'use strict'

var express = require('express')
var bodyParser = require('body-parser')
var morgan = require('morgan')
// var mongoose = require('mongoose')
var _ = require('lodash')
var async = require('async')
var CronJob = require('cron').CronJob
var TelegramBot = require('node-telegram-bot-api')

var log = require('log-to-file-and-console-node')
var MessageController = require('./controller/messageFacade')
var UsageController = require('./controller/usage')
var BotHandler = require('./route/botHandler')
var systemAdmin = require('./helper/jungBotSystemAdminHelper')

var app = express()
var bot = new TelegramBot(process.env.TELEGRAM_BOT_TOKEN, {polling: true})

MessageController.init()

app.use(morgan('combined', {'stream': log.stream}))
app.use(bodyParser.json())

var root = require('./route/root')
app.use('/', root)

bot.onText(/\/top(t|T)en/, function (msg) {
  BotHandler.onTopTen(msg, bot)
})

bot.onText(/\/all(j|J)ung/, function (msg) {
  BotHandler.onAllJung(msg, bot)
})

bot.onText(/\/jung(h|H)elp/, function (msg) {
  BotHandler.onHelp(msg, bot)
})

bot.on('message', function (msg) {
  BotHandler.onMessage(msg)
})

var debugFunction = function (msg) {
  bot.sendMessage(msg.chat.id, 'debug mode start')
  MessageController.getAllGroupIds().then(function (chatIds) {
    bot.sendMessage(msg.chat.id, 'getAllGroupIds, found: ' + chatIds.length)
    var groupCounter = 0
    var totalMessageCounter = 0
    var messageCountForGroupRegexp = /(?:^|\s)message: (.*?)(?:\s|$)/gm
    async.each(chatIds, function (chatId, callback) {
      var msg = {
        chat: {
          id: chatId
        }
      }
      log.i('chatId: ' + JSON.stringify(msg))
      MessageController.getTopTen(msg, true).then(function (message) {
        if (!_.isEmpty(message)) {
          log.i('message: \n\n' + message)
          groupCounter += 1
          try {
            var match = messageCountForGroupRegexp.exec(message)
            var totalMessage = Number(match[1])
            totalMessageCounter += totalMessage
            log.i('totalMessage: ' + totalMessage)
          } catch (e) {
            log.e('totalMessage error: ' + JSON.stringify(e))
          }
        }
        callback(null)
      })
    }, function (err) {
      log.i('debug mode end')
      if (!err) {
        bot.sendMessage(msg.chat.id,
          'debug mode end:\nget topTen for no. of groups: ' + groupCounter +
          '\ntotol no. of message in 7 days: ' + totalMessageCounter)
      } else {
        bot.sendMessage(msg.chat.id, err)
      }
    })
  })
}

bot.onText(/\/debug/, function (msg) {
  if (systemAdmin.isAdmin(msg)) {
    debugFunction(msg)
  }
})

var offJob = new CronJob({
  cronTime: '00 00 18 * * 1-5',
  onTick: function () {
    MessageController.getAllGroupIds().then(function (chatIds) {
      async.each(chatIds, function (chatId) {
        var msg = {
          chat: {
            id: chatId
          }
        }
        bot.sendMessage(chatId, '夠鐘收工~~')
        MessageController.getTopTen(msg, true).then(function (message) {
          if (!_.isEmpty(message)) {
            bot.sendMessage(chatId, message)
          }
        })
      })
    })
  },
  timeZone: 'Asia/Hong_Kong'
})
offJob.start()

var databaseMaintenance = function () {
  MessageController.cleanup()
  UsageController.cleanup()
}

var cleanupJob = new CronJob({
  cronTime: '0 0 0-17,19-23 * * *',
  onTick: function () {
    databaseMaintenance()
  },
  timeZone: 'Asia/Hong_Kong'
})
cleanupJob.start()

// cleanup when service start
databaseMaintenance()

module.exports = app
