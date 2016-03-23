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

var getJung = function(chatId, isAll) {
  var message = isAll ?
    'All 冗員s in the last 7 days:\n\n' :
    'Top 10 冗員s in the last 7 days:\n\n';
  var callback = function (err, results) {
    var total;
    if (err) {
      log.e('err: ' + JSON.stringify(err));
      bot.sendMessage(chatId, '[Error] ' + err.message);
    } else {
      for (var i = 0, l = results.length; i < l; i++) {
        total = results[i].total;
        message += results[i].firstName + ' ' + results[i].lastName + ' ' + results[i].percent + '\n';
      }
      if (total) {
        message += '\nTotal message: ' + total;
      }
      bot.sendMessage(chatId, message);
    }
  };
  if (isAll) {
    MessageController.getAllJung(chatId, callback);
  } else {
    MessageController.getTopTen(chatId, callback);
  }
};

bot.onText(/\/topTen/, function (msg, match) {
  log.i('/topTen: ' + JSON.stringify(msg));
  getJung(msg.chat.id.toString());
});

bot.onText(/\/allJung/, function (msg, match) {
  log.i('/allJung: ' + JSON.stringify(msg));
  getJung(msg.chat.id.toString(), true);
});

bot.on('message', function (msg) {
  log.i('msg: ' + JSON.stringify(msg));
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

if (process.env.DEFAULT_CRON_JOB_GROUP_ID) {
  var job = new CronJob({
    cronTime: '00 00 18 * * 1-5',
    onTick: function() {
      bot.sendMessage(process.env.DEFAULT_CRON_JOB_GROUP_ID, '夠鐘收工~~');
      getJung(process.env.DEFAULT_CRON_JOB_GROUP_ID);
    },
    start: false,
    timeZone: 'Asia/Hong_Kong'
  });
  job.start();
}

module.exports = app;