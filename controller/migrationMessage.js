'use strict';

var MongoMessage = require('./mongoMessage');
var DummyMessage = require('./dummyMessage');

exports.clearCachedLastSender = MongoMessage.clearCachedLastSender;

exports.setCachedLastSender = MongoMessage.setCachedLastSender;

exports.shouldAddMessage = MongoMessage.shouldAddMessage;

exports.addMessage = function (msg, callback) {
  var rez = MongoMessage.addMessage(msg, callback);

  // Chain the persist message to another data store.
  DummyMessage.addMessage(msg, function() {});
  return rez;
};

exports.getAllGroupIds = MongoMessage.getAllGroupIds;

exports.getAllJung = MongoMessage.getAllJung;

exports.getTopTen = MongoMessage.getTopTen;