require('dotenv').load();
var express = require('express');
var bodyParser = require('body-parser');
var morgan = require('morgan');
var moment = require('moment');
var log = require('log-to-file-and-console-node');

var TelegramBot = require('node-telegram-bot-api');

var app = express();
var bot = new TelegramBot(process.env.TELEGRAM_BOT_TOKEN, {polling: true});

app.use(morgan('combined', {'stream': log.stream}));
app.use(bodyParser.json());

bot.onText(/./, function (msg, match) {
  log.i('msg: ' + JSON.stringify(msg));
  bot.sendMessage(chatId, chatId);
});

app.route('/')
.get(function (req, res) {
  res.json({
    status: 'OK',
    desc: 'For UpTimeRobot'
  });
});

module.exports = app;
