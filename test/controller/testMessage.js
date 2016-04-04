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
          "__v": 0,
          "lastName": "stubLastName",
          "firstName": "stubFirstName",
          "username": "stubUsername",
          "userId": "stubFromId",
          "chatId": "stubChatId",
          "_id": "56fccf467b5633c02fb4eb7e",
          "dateCreated": "2016-03-31T07:18:30.806Z"
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

  // TODO: getAllJung

  // TODO: getTopTen

  // TODO: getAllGroupIds

});
