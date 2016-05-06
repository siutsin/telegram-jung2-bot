'use strict';

var Message = require('../model/message');
var Usage = require('../model/message');
var Constants = require('../model/constants');
require('moment');
var moment = require('moment-timezone');

var cachedLastSender = global.cachedLastSender;

var DummyDAO = function() {
	var _this = this;
	this.init = function() {
	};

	this.Message = Message;
	this.Usage = Usage;
	this.getCount = function (msg) {
		return 100;
	};

	this.getJung = function (msg, limit) {
	  return [{'firstName':'First', 'lastName':'Last', 'count':50, 'dateCreated':new Date().getTime()}];
	};
	this.addMessage = function (msg, callback) {
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
	  //message.save(callback);
	};
	this.getAllGroupIds = function () {
		return Promise.resolve([0]);
	};
	this.getCountAndGetJung = function (msg, limit) {
	    var total = _this.getCount();
	    var getJungResults = _this.getJung();
	    for (var i = 0, l = getJungResults.length; i < l; i++) {
	      getJungResults[i].total = total;
	      getJungResults[i].percent = ((getJungResults[i].count / total) * 100).toFixed(2) + '%';
	    }
	    return getJungResults;
	};

	this.getJungMessage = function (msg, limit, force) {
	  var message = limit ? Constants.MESSAGE.TOP_TEN_TITLE : Constants.MESSAGE.ALL_JUNG_TITLE;
		var results = _this.getCountAndGetJung(msg, limit);
		var total = '';
		for (var i = 0, l = results.length; i < l; i++) {
			total = results[i].total;
			message += (i + 1) + '. ' + results[i].firstName + ' ' + results[i].lastName + ' ' + results[i].percent +
					' (' + moment(results[i].dateCreated).fromNow() + ')' + '\n';
		}
		message += '\nTotal message: ' + total;
		return Promise.resolve(message);
	};
};

module.exports = new DummyDAO();