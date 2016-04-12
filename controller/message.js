'use strict';

var log = require('log-to-file-and-console-node');
var mongoose = require('mongoose');
var Message = require('../model/message');
var UsageController = require('./usage');
var Constants = require('../model/constants');
require('moment');
var moment = require('moment-timezone');
var _ = require('lodash');

var getCount = function (msg) {
  var promise = new mongoose.Promise();
  var chatId = msg.chat.id.toString();
  Message.count({
    chatId: chatId.toString(),
    dateCreated: {
      $gte: new Date(moment().subtract(7, 'day').toISOString())
    }
  }, function (err, total) {
    if (err) {
      promise.error(err);
    } else {
      promise.complete(total);
    }
  });
  return promise;
};

var getJung = function (msg, limit) {
  var promise = new mongoose.Promise();
  var chatId = msg.chat.id.toString();
  var query = Message.aggregate([
      {
        $match: {
          chatId: chatId.toString(),
          dateCreated: {
            $gte: new Date(moment().subtract(7, 'day').toISOString())
          }
        }
      },
      {
        $group: {
          _id: '$userId',
          username: {$last: '$username'},
          firstName: {$last: '$firstName'},
          lastName: {$last: '$lastName'},
          dateCreated: {$last: '$dateCreated'},
          count: {$sum: 1}
        }
      }])
    .sort('-count');
  if (limit > 0) {
    query = query.limit(limit);
  }
  query.exec(function (err, results) {
    if (err) {
      promise.error(err);
    } else {
      promise.complete(results);
    }
  });
  return promise;
};

var getCountAndGetJung = function (msg, limit) {
  var promises = [
    getCount(msg),
    getJung(msg, limit)
  ];
  return Promise.all(promises).then(function (results) {
    var total = results[0];
    var getJungResults = results[1];
    for (var i = 0, l = getJungResults.length; i < l; i++) {
      getJungResults[i].total = total;
      getJungResults[i].percent = ((getJungResults[i].count / total) * 100).toFixed(2) + '%';
    }
    return getJungResults;
  });
};

var getJungMessage = function (msg, limit, force) {
  var message = limit ?
    'Top 10 冗員s in the last 7 days (last 上水 time):\n\n' :
    'All 冗員s in the last 7 days (last 上水 time):\n\n';
  return UsageController.isAllowCommand(msg, force).then(function onSuccess() {
    var promises = [
      UsageController.addUsage(msg),
      getCountAndGetJung(msg, limit).then(function (results) {
        var total = '';
        for (var i = 0, l = results.length; i < l; i++) {
          total = results[i].total;
          message += (i + 1) + '. ' + results[i].firstName + ' ' + results[i].lastName + ' ' + results[i].percent +
            ' (' + moment(results[i].dateCreated).fromNow() + ')' + '\n';
        }
        message += '\nTotal message: ' + total;
        return message;
      })
    ];
    return Promise.all(promises).then(function (results) {
      var message = results[1];
      return message;
    });
  }, function onFailure(usage) {
    if (usage.notified) {
      message = '';
    } else {
      var oneMinutesLater = moment(usage.dateCreated).add(Constants.COMMAND_COOLDOWN_TIME, 'minute').tz('Asia/Hong_Kong');
      message = '[Error] Commands will be available ' + oneMinutesLater.fromNow() +
        ' (' + oneMinutesLater.format('h:mm:ss a') + ' HKT).';
    }
    return message;
  });
};

exports.shouldAddMessage = function (msg) {
  var promise = new mongoose.Promise();
  var chatId = msg.chat.id.toString();
  var userId = msg.from.id.toString();
  /*jshint camelcase: false */
  var isReplyingToMsg = !!msg.reply_to_message;
  /*jshint camelcase: true */
  Message.find({chatId: chatId.toString()})
    .sort('-dateCreated')
    .limit(1)
    .exec(function (err, messages) {
      if (!_.isEmpty(messages)) {
        var msg = messages[0];
        var result = isReplyingToMsg || (msg.userId !== userId);
        promise.complete(result);
      } else {
        promise.complete(true);
      }
    });
  return promise;
};

exports.getAllGroupIds = function (callback) {
  Message.find().distinct('chatId', callback);
};

exports.addMessage = function (msg, callback) {
  var message = new Message();
  message.chatId = msg.chat.id || '';
  message.userId = msg.from.id || '';
  message.username = msg.from.username || '';
  /*jshint camelcase: false */
  message.firstName = msg.from.first_name || '';
  message.lastName = msg.from.last_name || '';
  /*jshint camelcase: true */
  message.save(callback);
};

exports.getAllJung = function (msg) {
  return getJungMessage(msg);
};

exports.getTopTen = function (msg, force) {
  return getJungMessage(msg, 10, force);
};
