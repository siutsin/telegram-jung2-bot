'use strict';

require('chai').should();
var Router = require('../../route/router');
var MessageController = require('../../controller/message');
var log = require('log-to-file-and-console-node');
var _ = require('lodash');
var sinon = require('sinon');
require('sinon-as-promised');

describe('Router', function () {

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

  var stubBot = {
    sendMessage: function () {
    }
  };

  describe('onTopTen', function () {

    it('can handle /topten', function (done) {
      var sinonStub = sinon.stub(MessageController, 'getTopTen', function () {
        var stubPromise = sinon.stub().resolves('message');
        return stubPromise();
      });
      Router.onTopTen(stubMsg, stubBot);
      sinonStub.restore();
      done();
    });

    it('can handle empty message in /topten', function (done) {
      var sinonStub = sinon.stub(MessageController, 'getTopTen', function () {
        var stubPromise = sinon.stub().resolves('');
        return stubPromise();
      });
      Router.onTopTen(stubMsg, stubBot);
      sinonStub.restore();
      done();
    });

    it('can handle /topten err', function (done) {
      var sinonStub = sinon.stub(MessageController, 'getTopTen', function () {
        var stubPromise = sinon.stub().rejects(new Error('topTenError'));
        return stubPromise();
      });
      Router.onTopTen(stubMsg, stubBot);
      sinonStub.restore();
      done();
    });

  });

  describe('onAllJung', function () {

    it('can handle /allJung', function (done) {
      var sinonStub = sinon.stub(MessageController, 'getAllJung', function () {
        var stubPromise = sinon.stub().resolves('message');
        return stubPromise();
      });
      Router.onAllJung(stubMsg, stubBot);
      sinonStub.restore();
      done();
    });

    it('can handle empty message in /allJung', function (done) {
      var sinonStub = sinon.stub(MessageController, 'getAllJung', function () {
        var stubPromise = sinon.stub().resolves('');
        return stubPromise();
      });
      Router.onAllJung(stubMsg, stubBot);
      sinonStub.restore();
      done();
    });

    it('can handle /allJung err', function (done) {
      var sinonStub = sinon.stub(MessageController, 'getAllJung', function () {
        var stubPromise = sinon.stub().rejects(new Error('allJungError'));
        return stubPromise();
      });
      Router.onAllJung(stubMsg, stubBot);
      sinonStub.restore();
      done();
    });

  });

  describe('onMessage', function () {

    beforeEach(function () {
      MessageController.clearCachedLastSender();
    });

    it('can handle message', function (done) {
      var sinonStub = sinon.stub(MessageController, 'addMessage', function (msg, callback) {
        callback();
      });
      Router.onMessage(stubMsg);
      sinonStub.restore();
      done();
    });

    it('can skip repeated message', function (done) {
      MessageController.setCachedLastSender('stubChatId', 'stubFromId');
      Router.onMessage(stubMsg);
      done();
    });

  });

});