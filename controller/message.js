var log = require('log-to-file-and-console-node');
var Message = require('../model/message');
var moment = require('moment');

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

exports.getTopTen = function (callback) {
  var sevenDaysAgo = moment().subtract(7, 'day').toISOString();
  log.i(sevenDaysAgo);
  Message.aggregate(
    [
      {
        $match: {
          dateCreated: {
            $gte: new Date(sevenDaysAgo)
          }
        }
      },
      {
        $group: {
          _id: '$userId',
          count: {$sum: 1}
        }
      }
    ], callback);
};
