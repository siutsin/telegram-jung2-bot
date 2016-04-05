'use strict';

var log = require('log-to-file-and-console-node');
var Message = require('../model/message');
var moment = require('moment');
var _ = require('lodash');

var getJungFromDB = function (chatId, limit, callback) {
  var greaterThanOrEqualToSevenDaysQuery = {
    chatId: chatId.toString(),
    dateCreated: {
      $gte: new Date(moment().subtract(7, 'day').toISOString())
    }
  };
  Message.count(greaterThanOrEqualToSevenDaysQuery, function (err, total) {
    log.i('Number of messages: ' + total);
    var query = [
      {
        $match: greaterThanOrEqualToSevenDaysQuery
      },
      {
        $group: {
          _id: '$userId',
          username: {$last: '$username'},
          firstName: {$last: '$firstName'},
          lastName: {$last: '$lastName'},
          count: {$sum: 1}
        }
      },
      {
        $sort: {
          count: -1
        }
      }
    ];
    if (limit && limit > 0) {
      query.push({
        $limit: limit
      });
    }
    Message.aggregate(query, function (err, result) {
      if (!err && result && _.isArray(result)) {
        for (var i = 0, l = result.length; i < l; i++) {
          result[i].total = total;
          result[i].percent = ((result[i].count / total) * 100).toFixed(2) + '%';
        }
      }
      callback(err, result);
    });
  });
};

var getJungMessage = function (chatId, limit, callback) {
  var message = limit ?
    'Top 10 冗員s in the last 7 days:\n\n' :
    'All 冗員s in the last 7 days:\n\n';
  getJungFromDB(chatId, limit, function (err, results) {
    var total;
    if (!err) {
      for (var i = 0, l = results.length; i < l; i++) {
        total = results[i].total;
        message += (i + 1) + '. ' + results[i].firstName + ' ' + results[i].lastName + ' ' + results[i].percent + '\n';
      }
      if (total) {
        message += '\nTotal message: ' + total;
      }
      callback(message);
    }
  });
};

exports.isSameAsPreviousSender = function (chatId, userId, callback) {
  Message.find({})
    .where('chatId').equals(chatId.toString())
    .sort('-dateCreated')
    .limit(1)
    .exec(function (err, messages) {
      if (!err && messages && !_.isEmpty(messages)) {
        var msg = messages[0];
        var result = (msg.userId === userId);
        callback(result);
      } else {
        callback(false);
      }
    });
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

exports.getAllJung = function (chatId, callback) {
  getJungMessage(chatId, null, callback);
};

exports.getTopTen = function (chatId, callback) {
  getJungMessage(chatId, 10, callback);
};
