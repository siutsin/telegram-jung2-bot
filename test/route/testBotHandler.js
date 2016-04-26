'use strict';

require('chai').should();
var BotHandler = require('../../route/botHandler');
var MessageController = require('../../controller/message');
var PremierLeagueController = require('../../controller/premierLeague');
var log = require('log-to-file-and-console-node');
var _ = require('lodash');
var sinon = require('sinon');
require('sinon-as-promised');

describe('BotHandler', function () {

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
      BotHandler.onTopTen(stubMsg, stubBot);
      sinonStub.restore();
      done();
    });

    it('can handle empty message in /topten', function (done) {
      var sinonStub = sinon.stub(MessageController, 'getTopTen', function () {
        var stubPromise = sinon.stub().resolves('');
        return stubPromise();
      });
      BotHandler.onTopTen(stubMsg, stubBot);
      sinonStub.restore();
      done();
    });

    it('can handle /topten err', function (done) {
      var sinonStub = sinon.stub(MessageController, 'getTopTen', function () {
        var stubPromise = sinon.stub().rejects(new Error('topTenError'));
        return stubPromise();
      });
      BotHandler.onTopTen(stubMsg, stubBot);
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
      BotHandler.onAllJung(stubMsg, stubBot);
      sinonStub.restore();
      done();
    });

    it('can handle empty message in /allJung', function (done) {
      var sinonStub = sinon.stub(MessageController, 'getAllJung', function () {
        var stubPromise = sinon.stub().resolves('');
        return stubPromise();
      });
      BotHandler.onAllJung(stubMsg, stubBot);
      sinonStub.restore();
      done();
    });

    it('can handle /allJung err', function (done) {
      var sinonStub = sinon.stub(MessageController, 'getAllJung', function () {
        var stubPromise = sinon.stub().rejects(new Error('allJungError'));
        return stubPromise();
      });
      BotHandler.onAllJung(stubMsg, stubBot);
      sinonStub.restore();
      done();
    });

  });

  describe('onHelp', function () {

    it('can send help message', function (done) {
      BotHandler.onHelp(stubMsg, stubBot);
      done();
    });

  });

  describe('onJungPremierLeagueTable', function () {

    it('can handle /jungPremierLeagueTable', function (done) {
      var sinonStub = sinon.stub(PremierLeagueController, 'getTable', function () {
        var stubPromise = sinon.stub().resolves('message');
        return stubPromise();
      });
      BotHandler.onJungPremierLeagueTable(stubMsg, stubBot);
      sinonStub.restore();
      done();
    });

    it('can handle empty message in /jungPremierLeagueTable', function (done) {
      var sinonStub = sinon.stub(PremierLeagueController, 'getTable', function () {
        var stubPromise = sinon.stub().resolves('');
        return stubPromise();
      });
      BotHandler.onJungPremierLeagueTable(stubMsg, stubBot);
      sinonStub.restore();
      done();
    });

    it('can handle /jungPremierLeagueTable err', function (done) {
      var sinonStub = sinon.stub(PremierLeagueController, 'getTable', function () {
        var stubPromise = sinon.stub().rejects(new Error('jungPremierLeagueTableError'));
        return stubPromise();
      });
      BotHandler.onJungPremierLeagueTable(stubMsg, stubBot);
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
      BotHandler.onMessage(stubMsg);
      sinonStub.restore();
      done();
    });

    it('can skip repeated message', function (done) {
      MessageController.setCachedLastSender('stubChatId', 'stubFromId');
      BotHandler.onMessage(stubMsg);
      done();
    });

  });

});