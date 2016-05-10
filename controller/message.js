'use strict';
var ds = require('../ds/datastore');

var cachedLastSender = global.cachedLastSender;
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

exports.addMessage = ds.addMessage;

exports.getAllGroupIds = ds.getAllGroupIds;

exports.getAllJung = function (msg, force) {
  return ds.getJungMessage(msg, 0, force);
};

exports.getTopTen = function (msg, force) {
  return ds.getJungMessage(msg, 10, force);
};