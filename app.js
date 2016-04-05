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

bot.onText(/\/top(t|T)en/, function (msg, match) {
  var chatId = msg.chat.id.toString();
  MessageController.getTopTen(chatId, function (message) {
    bot.sendMessage(chatId, message);
  });
});

bot.onText(/\/all(j|J)ung/, function (msg, match) {
  var chatId = msg.chat.id.toString();
  MessageController.getAllJung(chatId, function (message) {
    bot.sendMessage(chatId, message);
  });
});

bot.on('message', function (msg) {
  log.i('msg: ' + JSON.stringify(msg));
  var chatId = msg.chat.id.toString();
  var userId = msg.from.id.toString();
  MessageController.isSameAsPreviousSender(chatId, userId, function (isSame) {
    if (!isSame) {
      MessageController.addMessage(msg);
    } else {
      log.e('isSameAsPreviousSender');
    }
  });
});

app.route('/')
  .get(function (req, res) {
    res.json({
      status: 'OK',
      desc: 'For UpTimeRobot'
    });
  });

var job = new CronJob({
  cronTime: '00 00 18 * * 1-5',
  onTick: function () {
    MessageController.getAllGroupIds(function (error, chatIds) {
      if (error) {
        log.e('cronJob error: ' + JSON.stringify(error));
      } else {
        for (var i = 0, l = chatIds.length; i < l; i++) {
          var chatId = chatIds[i];
          bot.sendMessage(chatId, '夠鐘收工~~');
          /*jshint loopfunc: true */
          MessageController.getTopTen(chatId, function (message) {
            bot.sendMessage(chatId, message);
          });
          /*jshint loopfunc: false */
        }
      }
    });
  },
  start: false,
  timeZone: 'Asia/Hong_Kong'
});
job.start();

module.exports = app;