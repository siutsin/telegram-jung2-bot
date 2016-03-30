'use strict';

var log = require('log-to-file-and-console-node');
var Message = require('../model/message');
var moment = require('moment');
var _ = require('lodash');

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

var getJung = function(chatId, limit, callback) {
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
          username: {$first: '$username'},
          firstName: {$first: '$firstName'},
          lastName: {$first: '$lastName'},
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

exports.getAllJung = function (chatId, callback) {
  getJung(chatId, null, callback);
};

exports.getTopTen = function (chatId, callback) {
  getJung(chatId, 10, callback);
};
