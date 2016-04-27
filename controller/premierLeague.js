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
          title: {$last: '$chatTitle'},
          count: {$sum: 1}
        }
      }])
    .sort('-count')
    .limit(Constants.CONFIG.PREMIER_LEAGUE_SIZE);
  query.exec(function (err, results) {
    if (err) {
      promise.error(err);
    } else {
      promise.complete(results);
    }
  });
  return promise;
};

var getCurrentGroupMessageCount = function (msg) {
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

var appendPromoteRelegateMessage = function (msg, leagueMessage, leagueTables, currentGroupMessageCount) {
  const indexOfRelegationZoneStart = Constants.CONFIG.PREMIER_LEAGUE_SIZE - Constants.CONFIG.PREMIER_LEAGUE_RELEGATION_ZONE_SIZE; // 17
  const indexOfRelegationZoneEnd = Constants.CONFIG.PREMIER_LEAGUE_SIZE - 1;
  var chatId = msg.chat.id.toString();
  var index = _.indexOf(leagueTables, _.find(leagueTables, function (group) {
    return group._id === chatId;
  }));
  var title = msg.chat.title;
  var numberOfMessageNeeded;
  if (index >= indexOfRelegationZoneStart && index <= indexOfRelegationZoneEnd) { // 17-19
    // relegation zone, show number of message needed in order to get rid of the relegation zone
    var numberOfMessageInLastSafeGroup = leagueTables[indexOfRelegationZoneStart - 1].count;
    numberOfMessageNeeded = (numberOfMessageInLastSafeGroup - currentGroupMessageCount) + 1;
    leagueMessage = leagueMessage + '\n' + numberOfMessageNeeded + ' messages required for ' + title + ' to stay in 冗超聯...';
  } else if (index >= 1 && index < indexOfRelegationZoneStart - 1) { // 1-16
    // mid table, show number of message needed in order to takeover the next group
    var titleOfPreviousGroup = leagueTables[index - 1].title;
    var numberOfMessageInPreviousGroup = leagueTables[index - 1].count;
    numberOfMessageNeeded = (numberOfMessageInPreviousGroup - currentGroupMessageCount) + 1;
    leagueMessage = leagueMessage + '\n' + numberOfMessageNeeded + ' messages required for ' + title + ' to takeover ' + titleOfPreviousGroup;
  } else if (index === 0 && leagueTables.length > 1) {
    // champion with more than one groups in the league, show number of message more than first runner-up group
    var titleOfFirstRunnerUpGroup = leagueTables[1].title;
    var numberOfMessageInFirstRunnerUpGroup = leagueTables[1].count;
    var numberOfMessageAbove = currentGroupMessageCount - numberOfMessageInFirstRunnerUpGroup;
    leagueMessage = leagueMessage + '\n' + 'CHAMPION!!! ' + numberOfMessageAbove + ' messages above ' + titleOfFirstRunnerUpGroup;
  } else if (index === 0 && leagueTables.length === 1) {
    // champion with only one group in the league, do not show message
    leagueMessage = leagueMessage + '\n';
  } else {
    // relegation zone, show number of message needed in order to get rid of the relegation zone
    numberOfMessageNeeded = (_.last(leagueTables).count - currentGroupMessageCount) + 1;
    leagueMessage = leagueMessage + '\n' + numberOfMessageNeeded + ' messages required for ' + title + ' to promote to 冗超聯...';
  }
  return leagueMessage;
};

var getTableMessage = function (msg) {
  var message = Constants.PREMIER_LEAGUE.TABLE_TITLE;
  return UsageController.isAllowCommand(msg).then(function onSuccess() {
    var promises = [
      UsageController.addUsage(msg),
      getTable().then(function (leagueTables) {
        for (var i = 0, l = leagueTables.length; i < l; i++) {
          message += (i + 1) + '. ' + leagueTables[i].title + ' ' + leagueTables[i].count + '\n';
        }
        return {message: message, leagueTables: leagueTables};
      }),
      getCurrentGroupMessageCount(msg)
    ];
    return Promise.all(promises).then(function (results) {
      var leagueMessage = results[1].message;
      var leagueTables = results[1].leagueTables;
      var currentGroupMessageCount = results[2];
      message = appendPromoteRelegateMessage(msg, leagueMessage, leagueTables, currentGroupMessageCount);
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