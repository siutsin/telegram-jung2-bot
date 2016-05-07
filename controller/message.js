'use strict';

var mongoose = require('mongoose');
var Message = require('../model/message');
var UsageController = require('./usage');
var Constants = require('../model/constants');
require('moment');
var moment = require('moment-timezone');

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

var getJungMessage;
getJungMessage = function (msg, limit, force) {
  var message = limit ? Constants.MESSAGE.TOP_TEN_TITLE : Constants.MESSAGE.ALL_JUNG_TITLE;
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
      return results[1];
    });
  }, function onFailure(usage) {
    if (usage.notified) {
      message = '';
    } else {
      var oneMinutesLater = moment(usage.dateCreated).add(Constants.CONFIG.COMMAND_COOLDOWN_TIME, 'minute').tz('Asia/Hong_Kong');
      message = '[Error] Commands will be available ' + oneMinutesLater.fromNow() +
        ' (' + oneMinutesLater.format('h:mm:ss a') + ' HKT).';
    }
    return message;
  });
};

var cachedLastSender = {
  //chatId: 'userId'
};

exports.clearCachedLastSender = function () {
  cachedLastSender = {};
};

exports.setCachedLastSender = function (chatId, userId) {
  cachedLastSender[chatId] = userId;
};

exports.shouldAddMessage = function (msg) {
  var result = true;
  var chatId = msg.chat.id.toString();
  var userId = msg.from.id.toString();
  /*jshint camelcase: false */
  var isReplyingToMsg = !!msg.reply_to_message;
  /*jshint camelcase: true */
  if (isReplyingToMsg || !cachedLastSender[chatId]) {
    cachedLastSender[chatId] = userId;
  } else if (cachedLastSender[chatId] === userId) {
    result = false;
  }
  return result;
};

exports.addMessage = function (msg, callback) {
  cachedLastSender[msg.chat.id] = msg.from.id.toString();
  var message = new Message();
  message.chatId = msg.chat.id || '';
  message.chatTitle = msg.chat.title || '';
  message.userId = msg.from.id || '';
  message.username = msg.from.username || '';
  /*jshint camelcase: false */
  message.firstName = msg.from.first_name || '';
  message.lastName = msg.from.last_name || '';
  /*jshint camelcase: true */
  message.save(callback);
};

exports.getAllGroupIds = function () {
  var promise = new mongoose.Promise();
  Message.find({
    dateCreated: {
      $gte: new Date(moment().subtract(7, 'day').toISOString())
    }}
  ).distinct('chatId', function (err, chatIds) {
    if (err) {
      promise.error(err);
    } else {
      promise.complete(chatIds);
    }
  });
  return promise;
};

exports.getAllJung = function (msg, force) {
  return getJungMessage(msg, 0, force);
};

exports.getTopTen = function (msg, force) {
  return getJungMessage(msg, 10, force);
};
