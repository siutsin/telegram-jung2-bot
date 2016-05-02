'use strict';

var express = require('express');
var bodyParser = require('body-parser');
var morgan = require('morgan');
var mongoose = require('mongoose');
var _ = require('lodash');
var CronJob = require('cron').CronJob;
var log = require('log-to-file-and-console-node');
var MessageController = require('./controller/message');
var BotHandler = require('./route/botHandler');
var TelegramBot = require('node-telegram-bot-api');
var async = require('async');

var app = express();
var bot = new TelegramBot(process.env.TELEGRAM_BOT_TOKEN, {polling: true});

var connectionString = '127.0.0.1:27017/jung2bot';
if (process.env.MONGODB_URL) {
  connectionString = process.env.MONGODB_URL + 'jung2bot';
} else if (process.env.OPENSHIFT_MONGODB_DB_PASSWORD) {
  connectionString = process.env.OPENSHIFT_MONGODB_DB_USERNAME + ':' +
    process.env.OPENSHIFT_MONGODB_DB_PASSWORD + '@' +
    process.env.OPENSHIFT_MONGODB_DB_HOST + ':' +
    process.env.OPENSHIFT_MONGODB_DB_PORT + '/' +
    process.env.OPENSHIFT_APP_NAME;
}
mongoose.connect(connectionString, {db: {nativeParser: true}});

app.use(morgan('combined', {'stream': log.stream}));
app.use(bodyParser.json());

var root = require('./route/root');
app.use('/', root);

bot.onText(/\/top(t|T)en/, function (msg, match) {
  BotHandler.onTopTen(msg, bot);
});

bot.onText(/\/all(j|J)ung/, function (msg, match) {
  BotHandler.onAllJung(msg, bot);
});

bot.onText(/\/help/, function (msg, match) {
  BotHandler.onHelp(msg, bot);
});

bot.onText(/\/jung(p|P)remier(l|L)eague/, function (msg, match) {
  BotHandler.onJungPremierLeagueTable(msg, bot);
});

// TODO: code refactoring required
var spamRecord = {};
var checkSpam = function (msg) {
  var chatId = msg.chat.id;
  if (!spamRecord[chatId]) {
    spamRecord[chatId] = 0;
  }
  spamRecord[chatId]++;
  // assume max conversation rate = 0.2 msg/s
  return spamRecord[chatId] < 720; // max 720 msg per hour in a group
};

var checkSpamJob = new CronJob({
  cronTime: '00 00 */1 * * *',
  onTick: function () {
    spamRecord = {};
  },
  start: true,
  timeZone: 'Asia/Hong_Kong'
});

bot.on('message', function (msg) {
  if (checkSpam(msg)) {
    BotHandler.onMessage(msg);
  }
});

var job = new CronJob({
  cronTime: '00 00 18 * * 1-5',
  //cronTime: '*/5 * * * * *',
  onTick: function () {
    MessageController.getAllGroupIds().then(function (chatIds) {
      async.each(chatIds, function (chatId, callback) {
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

module.exports = app;