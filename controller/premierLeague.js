'use strict';

var log = require('log-to-file-and-console-node');
var mongoose = require('mongoose');
var Message = require('../model/message');
var UsageController = require('./usage');
var Constants = require('../model/constants');
require('moment');
var moment = require('moment-timezone');
var _ = require('lodash');

var getTable = function () {
  var promise = new mongoose.Promise();
  var query = Message.aggregate([
      {
        $match: {
          dateCreated: {
            $gte: new Date(moment().subtract(7, 'day').toISOString())
          }
        }
      },
      {
        $group: {
          _id: '$chatId',
          title: {$last: '$title'},
          count: {$sum: 1}
        }
      }])
    .sort('-count')
    .limit(20);
  query.exec(function (err, results) {
    if (err) {
      promise.error(err);
    } else {
      promise.complete(results);
    }
  });
  return promise;
};

var getTableMessage = function (msg) {
  var message = Constants.PREMIER_LEAGUE.TABLE_TITLE;
  return UsageController.isAllowCommand(msg).then(function onSuccess() {
    var promises = [
      UsageController.addUsage(msg),
      getTable().then(function (results) {
        for (var i = 0, l = results.length; i < l; i++) {
          message += (i + 1) + '. ' + results[i].title + ' ' + results[i].count + '\n';
        }
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
      var oneMinutesLater = moment(usage.dateCreated).add(Constants.CONFIG.COMMAND_COOLDOWN_TIME, 'minute').tz('Asia/Hong_Kong');
      message = '[Error] Commands will be available ' + oneMinutesLater.fromNow() +
        ' (' + oneMinutesLater.format('h:mm:ss a') + ' HKT).';
    }
    return message;
  });
};

exports.getTable = function (msg) {
  return getTableMessage(msg);
};