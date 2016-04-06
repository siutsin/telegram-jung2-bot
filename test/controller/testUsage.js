'use strict';

require('chai').should();
var sinon = require('sinon');
require('sinon-mongoose');
var log = require('log-to-file-and-console-node');
var _ = require('lodash');

var UsageController = require('../../controller/usage');
var Usage = require('../../model/usage');

describe('UsageController', function () {

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

  describe('isAllowCommand', function () {

    it('should allow if time diff is greater than or equal to 3 mins', function (done) {
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
      UsageController.isAllowCommand(stubMsg).then(function onSuccess() {
        UsageMock.verify();
        UsageMock.restore();
        done();
      }, function onFailure() {
        false.should.equal(true); // should fail
        done();
      });
    });

    it('should not allow if time diff is smaller than 3 mins', function (done) {
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
      UsageController.isAllowCommand(stubMsg).then(function onSuccess() {
        false.should.equal(true); // should fail
        done();
      }, function onFailure() {
        UsageMock.verify();
        UsageMock.restore();
        done();
      });
    });

    it('should allow if no record', function (done) {
      var UsageMock = sinon.mock(Usage);
      UsageMock
        .expects('find').withArgs({chatId: 'stubChatId'})
        .chain('sort').withArgs('-dateCreated')
        .chain('limit').withArgs(1)
        .chain('exec')
        .yields(null, []);
      UsageController.isAllowCommand(stubMsg).then(function onSuccess() {
        UsageMock.verify();
        UsageMock.restore();
        done();
      }, function onFailure() {
        false.should.equal(true); // should fail
        done();
      });
    });

  });

});