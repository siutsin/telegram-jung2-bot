'use strict';

var Message = require('../model/message');
var Constants = require('../model/constants');
require('moment');
var moment = require('moment-timezone');


var cachedLastSender = {
  //chatId: 'userId'
};

exports.cachedLastSender = cachedLastSender;

exports.clearCachedLastSender = function () {
  cachedLastSender.length = 0;
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

exports.createMessage = function (msg, callback) {
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
  return message;
};
