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
var spamRecord = {
  // chatId: { hour: 0, day: 0, shouldNotify: true };
};
var checkSpam = function (msg, callback) {
  var chatId = msg.chat.id;
  if (!spamRecord[chatId]) {
    spamRecord[chatId] = { hour: 0, day: 0, shouldNotify: true };
  }
  spamRecord[chatId].hour++;
  spamRecord[chatId].day++;
  // max 720 msg per hour in a group
  // max 4000 msg per day in a group
  if (spamRecord[chatId].hour < 720 && spamRecord[chatId].day < 4000) {
    callback(true);
  } else {
    callback(false, spamRecord[chatId].shouldNotify);
    spamRecord[chatId].shouldNotify = false;
  }
};

var checkSpamHourlyJob = new CronJob({
  cronTime: '00 00 */1 * * *',
  onTick: function () {
    _(spamRecord).forEach(function(group) {
      group.hour = 0;
      group.shouldNotify = true;
    });
  },
  start: true,
  timeZone: 'Asia/Hong_Kong'
});

var checkSpamDailyJob = new CronJob({
  cronTime: '15 00 00 * * *',
  onTick: function () {
    spamRecord = {};
  },
  start: true,
  timeZone: 'Asia/Hong_Kong'
});

bot.on('message', function (msg) {
  checkSpam(msg, function (shouldAdd, shouldNotifySpam) {
    if (shouldAdd) {
      BotHandler.onMessage(msg);
    } else if (shouldNotifySpam) {
      // TODO: code refactoring required
      bot.sendMessage(msg.chat.id, 'Spam detected, allowance 720 msg per hour / 4000 msg per day');
      // TODO: add group to blacklist in db
    }
  });
});

var job = new CronJob({
  cronTime: '00 00 18 * * 1-5',
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