'use strict';

require('dotenv').load();
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

var app = express();
var bot = new TelegramBot(process.env.TELEGRAM_BOT_TOKEN, {polling: true});

var connectionString = '127.0.0.1:27017/telegram-jung2-bot';
if (process.env.OPENSHIFT_MONGODB_DB_PASSWORD) {
  connectionString = process.env.OPENSHIFT_MONGODB_DB_USERNAME + ':' +
    process.env.OPENSHIFT_MONGODB_DB_PASSWORD + '@' +
    process.env.OPENSHIFT_MONGODB_DB_HOST + ':' +
    process.env.OPENSHIFT_MONGODB_DB_PORT + '/' +
    process.env.OPENSHIFT_APP_NAME;
}
mongoose.connect(connectionString);

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

bot.on('message', function (msg) {
  BotHandler.onMessage(msg);
});

var job = new CronJob({
  cronTime: '00 00 18 * * 1-5',
  onTick: function () {
    MessageController.getAllGroupIds().then(function onSuccess(chatIds) {
      for (var i = 0, l = chatIds.length; i < l; i++) {
        const chatId = chatIds[i];
        var msg = {
          chat: {
            id: chatId
          }
        };
        bot.sendMessage(chatId, '夠鐘收工~~');
        /*jshint loopfunc: true */
        MessageController.getTopTen(msg, true).then(function onSuccess(message) {
          if (!_.isEmpty(message)) {
            bot.sendMessage(chatId, message);
          }
        });
        /*jshint loopfunc: false */
      }
    }, function onFailure(err) {
      log.e('cronJob error: ' + JSON.stringify(err));
    });
  },
  start: false,
  timeZone: 'Asia/Hong_Kong'
});
job.start();

module.exports = app;