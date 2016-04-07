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
  log.i('/topten msg: ' + JSON.stringify(msg));
  MessageController.getTopTen(msg).then(function onSuccess(message) {
    if (!_.isEmpty(message)) {
      bot.sendMessage(msg.chat.id, message);
    } else {
      log.e('/topten: message is empty');
    }
  }, function onFailure(err) {
    bot.sendMessage(msg.chat.id, err.message);
  });
});

bot.onText(/\/all(j|J)ung/, function (msg, match) {
  log.i('/alljung msg: ' + JSON.stringify(msg));
  MessageController.getAllJung(msg).then(function onSuccess(message) {
    if (!_.isEmpty(message)) {
      bot.sendMessage(msg.chat.id, message);
    } else {
      log.e('/alljung: message is empty');
    }
  }, function onFailure(err) {
    bot.sendMessage(msg.chat.id, err.message);
  });
});

bot.on('message', function (msg) {
  log.i('msg: ' + JSON.stringify(msg));
  MessageController.shouldAddMessage(msg).then(function (shouldAdd) {
    if (shouldAdd) {
      MessageController.addMessage(msg);
    } else {
      log.e('shouldAdd: ' + shouldAdd);
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
          const chatId = chatIds[i];
          var msg = {
            chat: {
              id: chatId
            }
          };
          /*jshint loopfunc: true */
          MessageController.getTopTen(msg, true).then(function onSuccess(message) {
            if (!_.isEmpty(message)) {
              bot.sendMessage(msg.chat.id, message);
            } else {
              log.e('/topten: message is empty');
            }
          }, function onFailure(err) {
            bot.sendMessage(msg.chat.id, err.message);
          }).then(function () {
            bot.sendMessage(chatId, '夠鐘收工~~');
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