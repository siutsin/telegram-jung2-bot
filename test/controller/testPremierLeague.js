'use strict';

require('chai').should();
var sinon = require('sinon');
require('sinon-mongoose');
var log = require('log-to-file-and-console-node');
var _ = require('lodash');
var Constants = require('../../model/constants');

var PremierLeagueController = require('../../controller/premierLeague');
var Message = require('../../model/message');
var Usage = require('../../model/usage');

describe('PremierLeagueController', function () {

  var stubMsg = {
    chat: {
      id: 'stubChatId',
      title: 'stubChatTitle'
    },
    from: {
      id: 'stubFromId',
      username: 'stubUsername',
      first_name: 'stubFirstName',
      last_name: 'stubLastName'
    }
  };

  describe('getTable', function () {

    it('can get all table within 7 days', function (done) {
      var UsageMock = sinon.mock(Usage);
      UsageMock
        .expects('find').withArgs({chatId: 'stubChatId'})
        .chain('sort').withArgs('-dateCreated')
        .chain('limit').withArgs(1)
        .chain('exec')
        .yields(null, [{
          chatId: 'stubChatId',
          notified: false,
          dateCreated: new Date('2016-01-01T0:00:00')
        }]);
      var findOneAndUpdateSinonStub = sinon.stub(Usage, 'findOneAndUpdate', function (conditions, update, options, callback) {
        callback( // err, foundObject
          null,
          {
            '__v': 0,
            'chatId': 'stubChatId',
            '_id': '5705c45f6c3467f672cdff50',
            'dateCreated': '2016-04-07T02:22:23.185Z',
            'notified': false
          }
        );
      });
      var usageSavesinonStub = sinon.stub(Usage.prototype, 'save', function (callback) {
        callback( // err, savedObject, numAffected
          null,
          {
            '__v': 0,
            'chatId': 'stubChatId',
            '_id': '5705c45f6c3467f672cdff50',
            'dateCreated': '2016-04-07T02:22:23.185Z',
            'notified': false
          },
          1);
      });
      var sinonAggregateStub = sinon.stub(Object.getPrototypeOf(Message), 'aggregate', function (query) {
        return {
          sort: function () {
            return {
              limit: function () {
                return {
                  exec: function (callback) {
                    callback(null, [{
                      "_id": "stubChatId",
                      "title": "stubChatTitle",
                      "count": 12
                    }]);
                  }
                }
              }
            }
          }
        };
      });
      PremierLeagueController.getTable(stubMsg).then(function onSuccess(message) {
        message.should.equal(Constants.PREMIER_LEAGUE.TABLE_TITLE + '1. stubChatTitle 12\n');
      }).catch(function (err) {
        false.should.equal(true); // should fail
      }).then(function always() {
        UsageMock.verify();
        UsageMock.restore();
        sinonAggregateStub.restore();
        findOneAndUpdateSinonStub.restore();
        usageSavesinonStub.restore();
        done();
      });
    });

    it('should return message indicating that the user should wait at least 1 mins for another command', function (done) {
      var UsageMock = sinon.mock(Usage);
      UsageMock
        .expects('find').withArgs({chatId: 'stubChatId'})
        .chain('sort').withArgs('-dateCreated')
        .chain('limit').withArgs(1)
        .chain('exec')
        .yields(null, [{
          chatId: 'stubChatId',
          notified: false,
          dateCreated: new Date()
        }]);
      var findOneAndUpdateSinonStub = sinon.stub(Usage, 'findOneAndUpdate', function (conditions, update, options, callback) {
        callback( // err, foundObject
          null,
          {
            '__v': 0,
            'chatId': 'stubChatId',
            '_id': '5705c45f6c3467f672cdff50',
            'dateCreated': '2016-04-07T02:22:23.185Z',
            'notified': false
          }
        );
      });
      var usageSavesinonStub = sinon.stub(Usage.prototype, 'save', function (callback) {
        callback( // err, savedObject, numAffected
          null,
          {
            '__v': 0,
            'chatId': 'stubChatId',
            '_id': '5705c45f6c3467f672cdff50',
            'dateCreated': '2016-04-07T02:22:23.185Z',
            'notified': false
          },
          1);
      });
      var sinonAggregateStub = sinon.stub(Object.getPrototypeOf(Message), 'aggregate', function (query) {
        var exec = function (callback) {
          callback(null, [{
            "_id": "stubFromId",
            "username": "stubUsername",
            "firstName": "stubFirstName",
            "lastName": "stubLastName",
            "count": 12
          }]);
        };
        var limit = function () {
          return {
            exec: exec
          }
        };
        var sort = function () {
          return {
            limit: limit
          }
        };
        return {
          sort: sort
        }
      });
      PremierLeagueController.getTable(stubMsg).then(function onSuccess(message) {
        (message.indexOf('[Error] Commands will be available in') >= 0).should.equal(true);
      }).catch(function (err) {
        false.should.equal(true); // should fail
      }).then(function always() {
        UsageMock.verify();
        UsageMock.restore();
        sinonAggregateStub.restore();
        findOneAndUpdateSinonStub.restore();
        usageSavesinonStub.restore();
        done();
      });
    });

    it('should block command request', function (done) {
      var UsageMock = sinon.mock(Usage);
      UsageMock
        .expects('find').withArgs({chatId: 'stubChatId'})
        .chain('sort').withArgs('-dateCreated')
        .chain('limit').withArgs(1)
        .chain('exec')
        .yields(null, [{
          chatId: 'stubChatId',
          notified: true,
          dateCreated: new Date()
        }]);
      var findOneAndUpdateSinonStub = sinon.stub(Usage, 'findOneAndUpdate', function (conditions, update, options, callback) {
        callback( // err, foundObject
          null,
          {
            '__v': 0,
            'chatId': 'stubChatId',
            '_id': '5705c45f6c3467f672cdff50',
            'dateCreated': '2016-04-07T02:22:23.185Z',
            'notified': false
          }
        );
      });
      var usageSavesinonStub = sinon.stub(Usage.prototype, 'save', function (callback) {
        callback( // err, savedObject, numAffected
          null,
          {
            '__v': 0,
            'chatId': 'stubChatId',
            '_id': '5705c45f6c3467f672cdff50',
            'dateCreated': '2016-04-07T02:22:23.185Z',
            'notified': false
          },
          1);
      });
      var sinonAggregateStub = sinon.stub(Object.getPrototypeOf(Message), 'aggregate', function (query) {
        var exec = function (callback) {
          callback(null, [{
            "_id": "stubFromId",
            "username": "stubUsername",
            "firstName": "stubFirstName",
            "lastName": "stubLastName",
            "count": 12
          }]);
        };
        var limit = function () {
          return {
            exec: exec
          }
        };
        var sort = function () {
          return {
            limit: limit
          }
        };
        return {
          sort: sort
        }
      });
      PremierLeagueController.getTable(stubMsg).then(function onSuccess(message) {
        (message === '').should.equal(true);
      }).catch(function (err) {
        false.should.equal(true); // should fail
      }).then(function always() {
        UsageMock.verify();
        UsageMock.restore();
        sinonAggregateStub.restore();
        findOneAndUpdateSinonStub.restore();
        usageSavesinonStub.restore();
        done();
      });
    });

    it('can handle error in aggregate', function (done) {
      var UsageMock = sinon.mock(Usage);
      UsageMock
        .expects('find').withArgs({chatId: 'stubChatId'})
        .chain('sort').withArgs('-dateCreated')
        .chain('limit').withArgs(1)
        .chain('exec')
        .yields(null, [{
          chatId: 'stubChatId',
          notified: false,
          dateCreated: new Date('2016-01-01T0:00:00')
        }]);
      var findOneAndUpdateSinonStub = sinon.stub(Usage, 'findOneAndUpdate', function (conditions, update, options, callback) {
        callback( // err, foundObject
          null,
          {
            '__v': 0,
            'chatId': 'stubChatId',
            '_id': '5705c45f6c3467f672cdff50',
            'dateCreated': '2016-04-07T02:22:23.185Z',
            'notified': false
          }
        );
      });
      var usageSavesinonStub = sinon.stub(Usage.prototype, 'save', function (callback) {
        callback( // err, savedObject, numAffected
          null,
          {
            '__v': 0,
            'chatId': 'stubChatId',
            '_id': '5705c45f6c3467f672cdff50',
            'dateCreated': '2016-04-07T02:22:23.185Z',
            'notified': false
          },
          1);
      });
      var sinonAggregateStub = sinon.stub(Object.getPrototypeOf(Message), 'aggregate', function (query) {
        var exec = function (callback) {
          callback(new Error('error'));
        };
        var limit = function () {
          return {
            exec: exec
          }
        };
        var sort = function () {
          return {
            limit: limit
          }
        };
        return {
          sort: sort
        }
      });
      PremierLeagueController.getTable(stubMsg).then(function onSuccess(message) {
        false.should.equal(true); // should fail
      }, function onFailure() {
        UsageMock.verify();
        UsageMock.restore();
        sinonAggregateStub.restore();
        findOneAndUpdateSinonStub.restore();
        usageSavesinonStub.restore();
        done();
      });
    });

  });

});