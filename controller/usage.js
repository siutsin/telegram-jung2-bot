'use strict';

var log = require('log-to-file-and-console-node');
var mongoose = require('mongoose');
var Usage = require('../model/usage');
var Constants = require('../model/constants');
require('moment');
var moment = require('moment-timezone');
var _ = require('lodash');

exports.addUsage = function (msg) {
  var usage = new Usage();
  usage.chatId = msg.chat.id || '';
  return usage.save();
};

var updateUsageNotice = function (chatId) {
  var promise = new mongoose.Promise();
  Usage.findOneAndUpdate(
    {chatId: chatId},
    {notified: true},
    {sort: '-dateCreated'},
    function callback(err, foundUsage) {
      promise.complete(foundUsage);
    }
  );
  return promise;
};

exports.isAllowCommand = function (msg, force) {
  var promise = new mongoose.Promise();
  if (force) {
    return promise.complete();
  }
  var chatId = msg.chat.id.toString();
  Usage.find({chatId: chatId.toString()})
    .sort('-dateCreated')
    .limit(1)
    .exec(function (err, usages) {
      if (_.isArray(usages) && !_.isEmpty(usages)) {
        var usage = usages[0];
        var diff = Math.abs(moment(usage.dateCreated).diff(moment(), 'minute', true));
        if (diff < Constants.CONFIG.COMMAND_COOLDOWN_TIME) {
          if (usage.notified) {
            promise.reject(usage);
          } else {
            updateUsageNotice(chatId).then(function () {
              promise.reject(usage);
            });
          }
        } else {
          promise.complete();
        }
      } else {
        promise.complete();
      }
    });
  return promise;
};
