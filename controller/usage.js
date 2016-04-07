'use strict';

var log = require('log-to-file-and-console-node');
var mongoose = require('mongoose');
var Usage = require('../model/usage');
var moment = require('moment');
var _ = require('lodash');

exports.addUsage = function (msg) {
  var usage = new Usage();
  usage.chatId = msg.chat.id || '';
  return usage.save();
};

exports.isAllowCommand = function (msg) {
  var promise = new mongoose.Promise();
  var chatId = msg.chat.id.toString();
  Usage.find({chatId: chatId.toString()})
    .sort('-dateCreated')
    .limit(1)
    .exec(function (err, usages) {
      if (_.isArray(usages) && !_.isEmpty(usages)) {
        var usage = usages[0];
        var diff = Math.abs(moment(usage.dateCreated).diff(moment(), 'minute', true));
        if (diff < 3) {
          promise.reject(moment(usage.dateCreated));
        } else {
          promise.complete();
        }
      } else {
        promise.complete();
      }
    });
  return promise;
};
