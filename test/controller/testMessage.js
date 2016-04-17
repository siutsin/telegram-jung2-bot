'use strict';

require('chai').should();
var sinon = require('sinon');
require('sinon-mongoose');
var log = require('log-to-file-and-console-node');
var _ = require('lodash');

var MessageController = require('../../controller/message');
var Message = require('../../model/message');
var Usage = require('../../model/usage');

describe('MessageController', function () {

  var stubMsg = {
    chat: {
      id: 'stubChatId'
    },
    from: {
      id: 'stubFromId',
      username: 'stubUsername',
      first_name: 'stubFirstName',
      last_name: 'stubLastName'
    }
  };

  describe('addMessage', function () {

    it('can save a message', function (done) {
      var sinonStub = sinon.stub(Message.prototype, 'save', function (callback) {
        callback( // err, product, numAffected
          null,
          {
            '__v': 0,
            'lastName': 'stubLastName',
            'firstName': 'stubFirstName',
            'username': 'stubUsername',
            'userId': 'stubFromId',
            'chatId': 'stubChatId',
            '_id': '56fccf467b5633c02fb4eb7e',
            'dateCreated': '2016-03-31T07:18:30.806Z'
          },
          1
        );
      });
      MessageController.addMessage(stubMsg, function (err, msg, numAffected) {
        (err === null).should.equal(true);
        (msg.userId).should.equal('stubFromId');
        (msg.chatId).should.equal('stubChatId');
        numAffected.should.equal(1);
        sinonStub.restore();
        done();
      });
    });

    it('can save a message with missing properties', function (done) {
      var stubEmptyMsg = {
        chat: {},
        from: {
          id: ''
        }
      };
      var sinonStub = sinon.stub(Message.prototype, 'save', function (callback) {
        callback( // err, product, numAffected
          null,
          {
            '__v': 0,
            'lastName': '',
            'firstName': '',
            'username': '',
            'userId': '',
            'chatId': '',
            '_id': '56fccf467b5633c02fb4eb7e',
            'dateCreated': '2016-03-31T07:18:30.806Z'
          },
          1
        );
      });
      MessageController.addMessage(stubEmptyMsg, function (err, msg, numAffected) {
        (err === null).should.equal(true);
        (msg.lastName).should.equal('');
        (msg.firstName).should.equal('');
        (msg.username).should.equal('');
        (msg.userId).should.equal('');
        (msg.chatId).should.equal('');
        numAffected.should.equal(1);
        sinonStub.restore();
        done();
      });
    });

  });

  describe('shouldAddMessage', function () {

    beforeEach(function () {
      MessageController.clearCachedLastSender();
    });

    it('allows adding msg if chatId is not exist', function (done) {
      MessageController.shouldAddMessage(stubMsg).should.equal(true);
      done();
    });

    it('allows adding msg if chatId\'s sender is not the same', function (done) {
      MessageController.setCachedLastSender('stubChatId', 'randomPeople');
      MessageController.shouldAddMessage(stubMsg).should.equal(true);
      done();
    });

    it('doest not allow adding msg if chatId\'s sender is same as msg.from.id', function (done) {
      MessageController.setCachedLastSender('stubChatId', 'stubFromId');
      MessageController.shouldAddMessage(stubMsg).should.equal(false);
      done();
    });

    it('allows adding replying msg even if chatId\'s sender is same as msg.from.id', function (done) {
      var stubReplyingMsg = {
        chat: {
          id: 'stubChatId'
        },
        from: {
          id: 'stubFromId',
          username: 'stubUsername',
          first_name: 'stubFirstName',
          last_name: 'stubLastName'
        },
        "reply_to_message": {
          "message_id": 123
        }
      };
      MessageController.setCachedLastSender('stubChatId', 'stubFromId');
      MessageController.shouldAddMessage(stubReplyingMsg).should.equal(true);
      done();
    });

  });

  describe('getAllGroupIds', function () {

    it('can get all group id within 7 days', function (done) {
      var MessageMock = sinon.mock(Message);
      MessageMock
        .expects('find')
        .chain('distinct').withArgs('chatId')
        .yields(null, ['123', '234']);
      MessageController.getAllGroupIds().then(function (chatIds) {
        MessageMock.verify();
        MessageMock.restore();
        _.isArray(chatIds).should.equal(true);
        chatIds[0].should.equal('123');
        chatIds[1].should.equal('234');
        done();
      });
    });

    it('can get handle error', function (done) {
      var MessageMock = sinon.mock(Message);
      MessageMock
        .expects('find')
        .chain('distinct').withArgs('chatId')
        .yields(new Error('getAllGroudIdsError'));
      MessageController.getAllGroupIds().then(function onSuccess() {
        false.should.equal(true); // should fail
      }, function onFailure(err) {
        MessageMock.verify();
        MessageMock.restore();
        err.message.should.equal('getAllGroudIdsError');
        done();
      });
    });

  });

  describe('getAllJung', function () {

    it('can get all yung', function (done) {
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
      var MessageMock = sinon.mock(Message);
      var sinonCountStub = sinon.stub(Message, 'count', function (err, callback) {
        callback(null, 123);
      });
      var sinonAggregateStub = sinon.stub(Object.getPrototypeOf(Message), 'aggregate', function (query) {
        return {
          sort: function () {
            return {
              exec: function (callback) {
                callback(null, [{
                  "_id": "stubFromId",
                  "username": "stubUsername",
                  "firstName": "stubFirstName",
                  "lastName": "stubLastName",
                  "count": 12
                }]);
              }
            }
          }
        };
      });
      MessageController.getAllJung(stubMsg).then(function onSuccess(message) {
        (message === 'All 冗員s in the last 7 days (last 上水 time):\n\n1. stubFirstName stubLastName 9.76% (a few seconds ago)\n\nTotal message: 123').should.equal(true);
      }, function onFailure() {
      }).catch(function (err) {
        false.should.equal(true); // should fail
      }).then(function always() {
        MessageMock.verify();
        MessageMock.restore();
        UsageMock.verify();
        UsageMock.restore();
        sinonCountStub.restore();
        sinonAggregateStub.restore();
        findOneAndUpdateSinonStub.restore();
        usageSavesinonStub.restore();
        done();
      });
    });

  });

  describe('getTopTen', function () {

    it('can get top ten', function (done) {
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
      var MessageMock = sinon.mock(Message);
      var sinonCountStub = sinon.stub(Message, 'count', function (err, callback) {
        callback(null, 123);
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
      MessageController.getTopTen(stubMsg).then(function onSuccess(message) {
        (message === 'Top 10 冗員s in the last 7 days (last 上水 time):\n\n1. stubFirstName stubLastName 9.76% (a few seconds ago)\n\nTotal message: 123').should.equal(true);
        UsageMock.verify();
        UsageMock.restore();
        findOneAndUpdateSinonStub.restore();
        usageSavesinonStub.restore();
        MessageMock.verify();
        MessageMock.restore();
        sinonCountStub.restore();
        sinonAggregateStub.restore();
        done();
      }, function onFailure(err) {
        false.should.equal(true); // should fail
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
      var MessageMock = sinon.mock(Message);
      var sinonCountStub = sinon.stub(Message, 'count', function (err, callback) {
        callback(null, 123);
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
      MessageController.getTopTen(stubMsg).then(function onSuccess(message) {
        (message === '').should.equal(true);
        UsageMock.verify();
        UsageMock.restore();
        findOneAndUpdateSinonStub.restore();
        usageSavesinonStub.restore();
        MessageMock.verify();
        MessageMock.restore();
        sinonCountStub.restore();
        sinonAggregateStub.restore();
        done();
      }, function onFailure(err) {
        false.should.equal(true); // should fail
        done();
      });
    });

    it('should return message indicating that the user should wait at least 3 mins for another command', function (done) {
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
      var MessageMock = sinon.mock(Message);
      var sinonCountStub = sinon.stub(Message, 'count', function (err, callback) {
        callback(null, 123);
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
      MessageController.getTopTen(stubMsg).then(function onSuccess(message) {
        (message.indexOf('[Error] Commands will be available in') >= 0).should.equal(true);
        UsageMock.verify();
        UsageMock.restore();
        findOneAndUpdateSinonStub.restore();
        usageSavesinonStub.restore();
        MessageMock.verify();
        MessageMock.restore();
        sinonCountStub.restore();
        sinonAggregateStub.restore();
        done();
      }, function onFailure(err) {
        false.should.equal(true); // should fail
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
      var MessageMock = sinon.mock(Message);
      var sinonCountStub = sinon.stub(Message, 'count', function (err, callback) {
        callback(null, 123);
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
      MessageController.getTopTen(stubMsg).then(function onSuccess(message) {
        false.should.equal(true); // should fail
      }, function onFailure (err) {
        err.message.should.equal('error');
        UsageMock.verify();
        UsageMock.restore();
        findOneAndUpdateSinonStub.restore();
        usageSavesinonStub.restore();
        MessageMock.verify();
        MessageMock.restore();
        sinonCountStub.restore();
        sinonAggregateStub.restore();
        done();
      });
    });

    it('can handle error in count', function (done) {
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
      var MessageMock = sinon.mock(Message);
      var sinonCountStub = sinon.stub(Message, 'count', function (err, callback) {
        callback(new Error('anotherError'));
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
      MessageController.getTopTen(stubMsg).then(function onSuccess(message) {
        false.should.equal(true); // should fail
        done();
      }, function onFailure (err) {
        err.message.should.equal('anotherError');
        UsageMock.verify();
        UsageMock.restore();
        findOneAndUpdateSinonStub.restore();
        usageSavesinonStub.restore();
        MessageMock.verify();
        MessageMock.restore();
        sinonCountStub.restore();
        sinonAggregateStub.restore();
        done();
      });
    });

  });

});