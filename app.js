require('dotenv').load();
var express = require('express');
var bodyParser = require('body-parser');
var morgan = require('morgan');
var mongoose = require('mongoose');
var _ = require('lodash');
var log = require('log-to-file-and-console-node');
var MessageController = require('./controller/message');
var TelegramBot = require('node-telegram-bot-api');

var app = express();
var bot = new TelegramBot(process.env.TELEGRAM_BOT_TOKEN, {polling: true});

var connectionString = '127.0.0.1:27017/telegram-jung2-bot';
if(process.env.OPENSHIFT_MONGODB_DB_PASSWORD){
  connectionString = process.env.OPENSHIFT_MONGODB_DB_USERNAME + ':' +
    process.env.OPENSHIFT_MONGODB_DB_PASSWORD + '@' +
    process.env.OPENSHIFT_MONGODB_DB_HOST + ':' +
    process.env.OPENSHIFT_MONGODB_DB_PORT + '/' +
    process.env.OPENSHIFT_APP_NAME;
}
mongoose.connect(connectionString);

app.use(morgan('combined', {'stream': log.stream}));
app.use(bodyParser.json());

bot.onText(/\/topTen/, function (msg, match) {
  MessageController.getTopTen(function (err, topTens) {
    if (err) {
      log.e('err: ' + JSON.stringify(err));
      bot.sendMessage(msg.chat.id, '[Error] ' + err.message);
    } else {
      //log.i('topTens: ' + JSON.stringify(topTens));
      log.i(topTens);
      //bot.sendMessage(msg.chat.id, JSON.stringify(topTens));
    }
  });
});

bot.on('message', function (msg) {
  log.i('msg: ' + JSON.stringify(msg));
  // skip command
  if (_.isString(msg.text) && msg.text.match(/^\//)) {
    return;
  }
  MessageController.addMessage(msg, function (err) {
    if (err) {
      log.e('err: ' + JSON.stringify(err));
      bot.sendMessage(msg.chat.id, '[Error] ' + err.message);
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

module.exports = app;