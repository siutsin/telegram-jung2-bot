'use strict';

var mongoose = require('mongoose');
var MessageClass = require('../model/message');
var cacheConnection;
var MessageCache;
var persistenceConnection;
var MessagePersistence;

var UsageController = require('./usage');
var Constants = require('../model/constants');
require('moment');
var moment = require('moment-timezone');
var log = require('log-to-file-and-console-node');
var async = require('async');

UsageController.init();

var getCount = function (msg) {
  var promise = new mongoose.Promise();
  var chatId = msg.chat.id.toString();
  MessageCache.count({
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
  var query = MessageCache.aggregate([
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

// TODO: refactoring required
exports.init = function () {
  var connectionStringCache = '127.0.0.1:27017/jung2botCache';
  if (process.env.OPENSHIFT_MONGODB_DB_PASSWORD) {
    connectionStringCache = process.env.OPENSHIFT_MONGODB_DB_USERNAME + ':' +
      process.env.OPENSHIFT_MONGODB_DB_PASSWORD + '@' +
      process.env.OPENSHIFT_MONGODB_DB_HOST + ':' +
      process.env.OPENSHIFT_MONGODB_DB_PORT + '/' +
      process.env.OPENSHIFT_APP_NAME;
  }

  var connectionStringPersistence = '127.0.0.1:27017/jung2bot';
  if (process.env.MONGODB_URL) {
    connectionStringPersistence = process.env.MONGODB_URL;
  }

  cacheConnection = mongoose.createConnection(connectionStringCache);
  persistenceConnection = mongoose.createConnection(connectionStringPersistence);
  MessageCache = cacheConnection.model('Message', MessageClass.getSchema());
  MessagePersistence = persistenceConnection.model('Message', MessageClass.getSchema());
};

var getJungMessage = function (msg, limit, force) {
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
exports.getJungMessage = getJungMessage;

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

var saveToMessageCache = function (msg) {
  var msgCache = new MessageCache();
  msgCache.chatId = msg.chat.id || '';
  msgCache.chatTitle = msg.chat.title || '';
  msgCache.userId = msg.from.id || '';
  msgCache.username = msg.from.username || '';
  /*jshint camelcase: false */
  msgCache.firstName = msg.from.first_name || '';
  msgCache.lastName = msg.from.last_name || '';
  /*jshint camelcase: true */
  return msgCache.save();
};

var saveToMessagePersistence = function (msg) {
  var msgPersistence = new MessagePersistence();
  msgPersistence.chatId = msg.chat.id || '';
  msgPersistence.chatTitle = msg.chat.title || '';
  msgPersistence.userId = msg.from.id || '';
  msgPersistence.username = msg.from.username || '';
  /*jshint camelcase: false */
  msgPersistence.firstName = msg.from.first_name || '';
  msgPersistence.lastName = msg.from.last_name || '';
  /*jshint camelcase: true */
  return msgPersistence.save();
};

exports.addMessage = function (msg, callback) {
  cachedLastSender[msg.chat.id] = msg.from.id.toString();
  var promises = [
    saveToMessageCache(msg),
    saveToMessagePersistence(msg)
  ];
  return Promise.all(promises).then(callback);
};

exports.getAllGroupIds = function () {
  var promise = new mongoose.Promise();
  MessageCache.find({
      dateCreated: {
        $gte: new Date(moment().subtract(7, 'day').toISOString())
      }
    }
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

exports.cleanup = function () {
  const numberToDelete = 10000;
  var shouldRepeat = true;
  var promise = new mongoose.Promise();
  async.whilst(
    function test() {
      return shouldRepeat;
    },
    function iteratee(next) {
      MessageCache.find({
        dateCreated: {
          $lt: new Date(moment().subtract(7, 'day').toISOString())
        }
      }).select('_id')
        .sort({_id: 1})
        .limit(numberToDelete)
        .exec(function (err, docs) {
            if (err) {
              log.e(err);
              next(err);
            } else {
              var ids = docs.map(function (doc) {
                return doc._id;
              });
              MessageCache.remove({_id: {$in: ids}}, function (err, result) {
                if (err) {
                  log.e(err);
                  next(err);
                } else {
                  var numberDeleted = result.result.n;
                  log.i('cleanup message cache database, numberDeleted: ' + numberDeleted);
                  shouldRepeat = (numberDeleted === numberToDelete);
                  next();
                }
              });
            }
          }
        );
    },
    function callback(err) {
      if (err) {
        log.e(err);
        promise.error(err);
      } else {
        promise.complete();
      }
    });
  return promise;
};
