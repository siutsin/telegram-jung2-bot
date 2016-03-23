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

/*
var sampleMsg = {
  "message_id": 10,
    "from": {
  "id": 12345,
      "first_name": "Simon",
      "last_name": "Li",
      "username": "simonli"
},
  "chat": {
  "id": 23456,
      "title": "Bot Testing",
      "type": "group"
},
  "date": 1458705792,
    "text": "yo"
};
*/
bot.on('message', function (msg) {
  log.i('msg: ' + JSON.stringify(msg));
  bot.sendMessage(msg.chat.id, 'hi');
});

app.route('/')
.get(function (req, res) {
  res.json({
    status: 'OK',
    desc: 'For UpTimeRobot'
  });
});

module.exports = app;
