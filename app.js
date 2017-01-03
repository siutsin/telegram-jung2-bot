'use strict';

var express = require('express');
var bodyParser = require('body-parser');
var morgan = require('morgan');
var mongoose = require('mongoose');
var _ = require('lodash');
var CronJob = require('cron').CronJob;
var log = require('log-to-file-and-console-node');
var MessageController = require('./controller/messageFacade');
var UsageController = require('./controller/usage');
var BotHandler = require('./route/botHandler');
var TelegramBot = require('node-telegram-bot-api');
var async = require('async');

var app = express();
var bot = new TelegramBot(process.env.TELEGRAM_BOT_TOKEN, {polling: true});

MessageController.init();

app.use(morgan('combined', {'stream': log.stream}));
app.use(bodyParser.json());

var root = require('./route/root');
app.use('/', root);

bot.onText(/\/top(t|T)en/, function (msg) {
  BotHandler.onTopTen(msg, bot);
});

bot.onText(/\/all(j|J)ung/, function (msg) {
  BotHandler.onAllJung(msg, bot);
});

bot.onText(/\/help/, function (msg) {
  BotHandler.onHelp(msg, bot);
});

bot.on('message', function (msg) {
  BotHandler.onMessage(msg);
});

var debugFunction = function (msg) {
  bot.sendMessage(msg.chat.id, 'debug mode start');
  MessageController.getAllGroupIds().then(function (chatIds) {
    bot.sendMessage(msg.chat.id, 'getAllGroupIds, found: ' + chatIds.length);
    var counter = 0;
    async.each(chatIds, function (chatId, callback) {
      var msg = {
        chat: {
          id: chatId
        }
      };
      log.i('chatId: ' + JSON.stringify(msg));
      MessageController.getTopTen(msg, true).then(function (message) {
        if (!_.isEmpty(message)) {
          counter += 1;
          log.i('message: \n\n' + message);
        }
        callback(null);
      });
    }, function (err) {
      log.i('debug mode end');
      if (!err) {
        bot.sendMessage(msg.chat.id, 'debug mode end, get topTen for no. of groups: ' + counter);
      } else {
        bot.sendMessage(msg.chat.id, err);
      }
    });
  });
};

bot.onText(/\/debug/, function (msg) {
  var adminList = process.env.ADMIN_ID.split(',');
  if (msg && msg.from && String(msg.from.id) && _.includes(adminList, String(msg.from.id))) {
    debugFunction(msg);
  }
});

var offJob = new CronJob({
  cronTime: '00 00 18 * * 1-5',
  onTick: function () {
    MessageController.getAllGroupIds().then(function (chatIds) {
      async.each(chatIds, function (chatId) {
        var msg = {
          chat: {
            id: chatId
          }
        };
        bot.sendMessage(chatId, '夠鐘收工~~');
        MessageController.getTopTen(msg, true).then(function (message) {
          if (!_.isEmpty(message)) {
            bot.sendMessage(chatId, message);
          }
        });
      });
    });
  },
  start: true,
  timeZone: 'Asia/Hong_Kong'
});

var databaseMaintenance = function () {
  MessageController.cleanup();
  UsageController.cleanup();
};

var cleanupJob = new CronJob({
  cronTime: '0 0 4 * * *',
  onTick: function () {
    databaseMaintenance();
  },
  start: true,
  timeZone: 'Asia/Hong_Kong'
});

// cleanup when service start
databaseMaintenance();

module.exports = app;
