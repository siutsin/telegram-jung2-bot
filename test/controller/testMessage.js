'use strict';

require('chai').should();
var sinon = require('sinon');
require('sinon-mongoose');
var log = require('log-to-file-and-console-node');

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

  });

  // TODO: getAllJung

  // TODO: getTopTen

  // TODO: getAllGroupIds

});
