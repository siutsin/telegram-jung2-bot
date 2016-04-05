'use strict';

require('chai').should();
var sinon = require('sinon');
require('sinon-mongoose');
var log = require('log-to-file-and-console-node');
var _ = require('lodash');

var MessageController = require('../../controller/message');
var Message = require('../../model/message');

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

  describe('shouldAddMessage', function () {

    it('can determine the record should not be added if the user id is same', function (done) {
      var MessageMock = sinon.mock(Message);
      MessageMock
        .expects('find').withArgs({chatId: 'stubChatId'})
        .chain('sort').withArgs('-dateCreated')
        .chain('limit').withArgs(1)
        .chain('exec')
        .yields(null, [{
          'userId': 'stubFromId'
        }]);
      MessageController.shouldAddMessage(stubMsg, function (result) {
        MessageMock.verify();
        MessageMock.restore();
        result.should.equal(false);
        done();
      });
    });

    it('can determine the record should be added if the user id is not the same', function (done) {
      var MessageMock = sinon.mock(Message);
      MessageMock
        .expects('find').withArgs({chatId: 'stubChatId'})
        .chain('sort').withArgs('-dateCreated')
        .chain('limit').withArgs(1)
        .chain('exec')
        .yields(null, [{
          'userId': 'anotherId'
        }]);
      MessageController.shouldAddMessage(stubMsg, function (result) {
        MessageMock.verify();
        MessageMock.restore();
        result.should.equal(true);
        done();
      });
    });

    it('can determine the record should be added if it is replying to message even if the sender id is the same', function (done) {
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
      var MessageMock = sinon.mock(Message);
      MessageMock
        .expects('find').withArgs({chatId: 'stubChatId'})
        .chain('sort').withArgs('-dateCreated')
        .chain('limit').withArgs(1)
        .chain('exec')
        .yields(null, [{
          'userId': 'stubFromId'
        }]);
      MessageController.shouldAddMessage(stubReplyingMsg, function (result) {
        MessageMock.verify();
        MessageMock.restore();
        result.should.equal(true);
        done();
      });
    });

    it('allows to add record if encountered error', function (done) {
      var MessageMock = sinon.mock(Message);
      MessageMock
        .expects('find').withArgs({chatId: 'stubChatId'})
        .chain('sort').withArgs('-dateCreated')
        .chain('limit').withArgs(1)
        .chain('exec')
        .yields(new Error('someError'));
      MessageController.shouldAddMessage(stubMsg, function (result) {
        MessageMock.verify();
        MessageMock.restore();
        result.should.equal(true);
        done();
      });
    });

  });

  it('can get all group id', function (done) {
    var MessageMock = sinon.mock(Message);
    MessageMock
      .expects('find')
      .chain('distinct').withArgs('chatId')
      .yields(null, ['123', '234']);
    MessageController.getAllGroupIds(function (err, result) {
      MessageMock.verify();
      MessageMock.restore();
      _.isArray(result).should.equal(true);
      result[0].should.equal('123');
      result[1].should.equal('234');
      done();
    });
  });

  describe('getAllJung', function () {

    it('can get all yung', function (done) {
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
        (message === 'All 冗員s in the last 7 days:\n\n1. stubFirstName stubLastName 9.76%\n\nTotal message: 123').should.equal(true);
      }).catch(function (err) {
        log.e('getAllJung err: ' + err.message);
        false.should.equal(true); // should fail
      }).then(function always() {
        MessageMock.verify();
        MessageMock.restore();
        sinonCountStub.restore();
        sinonAggregateStub.restore();
        done();
      });
    });

  });

  describe('getTopTen', function () {

    it('can get all yung', function (done) {
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
        (message === 'Top 10 冗員s in the last 7 days:\n\n1. stubFirstName stubLastName 9.76%\n\nTotal message: 123').should.equal(true);
      }).catch(function (err) {
        log.e('getTopTen err: ' + err.message);
        false.should.equal(true); // should fail
      }).then(function always() {
        MessageMock.verify();
        MessageMock.restore();
        sinonCountStub.restore();
        sinonAggregateStub.restore();
        done();
      });
    });

    it('can handle error in aggregate', function (done) {
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
      }).catch(function (err) {
        err.message.should.equal('error');
      }).then(function always() {
        MessageMock.verify();
        MessageMock.restore();
        sinonCountStub.restore();
        sinonAggregateStub.restore();
        done();
      });
    });

    it('can handle error in count', function (done) {
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
      }).catch(function (err) {
        err.message.should.equal('anotherError');
      }).then(function always() {
        MessageMock.verify();
        MessageMock.restore();
        sinonCountStub.restore();
        sinonAggregateStub.restore();
        done();
      });
    });

  });

})
;
