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

    it('can get league table within 7 days for non-league group', function (done) {
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
      var sinonCountStub = sinon.stub(Message, 'count', function (err, callback) {
        callback(null, 12);
      });
      var sinonAggregateStub = sinon.stub(Object.getPrototypeOf(Message), 'aggregate', function (query) {
        var stubArray = [];
        for (var i = 0, l = 20; i < l; i++) {
          var stub = {
            "_id": "stubChatId" + i,
            "title": "stubChatTitle" + i,
            "count": 100 - i
          };
          stubArray.push(stub);
        }
        return {
          sort: function () {
            return {
              limit: function () {
                return {
                  exec: function (callback) {
                    callback(null, stubArray);
                  }
                }
              }
            }
          }
        };
      });
      PremierLeagueController.getTable(stubMsg).then(function onSuccess(message) {
        (message.indexOf(Constants.PREMIER_LEAGUE.TABLE_TITLE) >= 0).should.equal(true);
        (message.indexOf('to promote to 冗超聯') >= 0).should.equal(true);
      }).catch(function (err) {
        false.should.equal(true); // should fail
      }).then(function always() {
        UsageMock.verify();
        UsageMock.restore();
        sinonCountStub.restore();
        sinonAggregateStub.restore();
        findOneAndUpdateSinonStub.restore();
        usageSavesinonStub.restore();
        done();
      });
    });

    it('can get league table within 7 days for relegating group', function (done) {
      var UsageMock = sinon.mock(Usage);
      UsageMock
        .expects('find').withArgs({chatId: 'stubChatId18'})
        .chain('sort').withArgs('-dateCreated')
        .chain('limit').withArgs(1)
        .chain('exec')
        .yields(null, [{
          chatId: 'stubChatId18',
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
      var sinonCountStub = sinon.stub(Message, 'count', function (err, callback) {
        callback(null, 83);
      });
      var sinonAggregateStub = sinon.stub(Object.getPrototypeOf(Message), 'aggregate', function (query) {
        var stubArray = [];
        for (var i = 0, l = 20; i < l; i++) {
          var stub = {
            "_id": "stubChatId" + i,
            "title": "stubChatTitle" + i,
            "count": 100 - i
          };
          stubArray.push(stub);
        }
        return {
          sort: function () {
            return {
              limit: function () {
                return {
                  exec: function (callback) {
                    callback(null, stubArray);
                  }
                }
              }
            }
          }
        };
      });
      PremierLeagueController.getTable({
        chat: {
          id: 'stubChatId18',
          title: 'stubChatTitle'
        },
        from: {
          id: 'stubFromId',
          username: 'stubUsername',
          first_name: 'stubFirstName',
          last_name: 'stubLastName'
        }
      }).then(function onSuccess(message) {
        (message.indexOf(Constants.PREMIER_LEAGUE.TABLE_TITLE) >= 0).should.equal(true);
        (message.indexOf('to stay in 冗超聯...') >= 0).should.equal(true);
      }).catch(function (err) {
        false.should.equal(true); // should fail
      }).then(function always() {
        UsageMock.verify();
        UsageMock.restore();
        sinonCountStub.restore();
        sinonAggregateStub.restore();
        findOneAndUpdateSinonStub.restore();
        usageSavesinonStub.restore();
        done();
      });
    });

    it('can get league table within 7 days for mid-table group', function (done) {
      var UsageMock = sinon.mock(Usage);
      UsageMock
        .expects('find').withArgs({chatId: 'stubChatId10'})
        .chain('sort').withArgs('-dateCreated')
        .chain('limit').withArgs(1)
        .chain('exec')
        .yields(null, [{
          chatId: 'stubChatId10',
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
      var sinonCountStub = sinon.stub(Message, 'count', function (err, callback) {
        callback(null, 91);
      });
      var sinonAggregateStub = sinon.stub(Object.getPrototypeOf(Message), 'aggregate', function (query) {
        var stubArray = [];
        for (var i = 0, l = 20; i < l; i++) {
          var stub = {
            "_id": "stubChatId" + i,
            "title": "stubChatTitle" + i,
            "count": 100 - i
          };
          stubArray.push(stub);
        }
        return {
          sort: function () {
            return {
              limit: function () {
                return {
                  exec: function (callback) {
                    callback(null, stubArray);
                  }
                }
              }
            }
          }
        };
      });
      PremierLeagueController.getTable({
        chat: {
          id: 'stubChatId10',
          title: 'stubChatTitle10'
        },
        from: {
          id: 'stubFromId',
          username: 'stubUsername',
          first_name: 'stubFirstName',
          last_name: 'stubLastName'
        }
      }).then(function onSuccess(message) {
        (message.indexOf(Constants.PREMIER_LEAGUE.TABLE_TITLE) >= 0).should.equal(true);
        (message.indexOf(' to takeover') >= 0).should.equal(true);
      }).catch(function (err) {
        false.should.equal(true); // should fail
      }).then(function always() {
        UsageMock.verify();
        UsageMock.restore();
        sinonCountStub.restore();
        sinonAggregateStub.restore();
        findOneAndUpdateSinonStub.restore();
        usageSavesinonStub.restore();
        done();
      });
    });

    it('can get league table within 7 days for the first group with more than 2 groups', function (done) {
      var UsageMock = sinon.mock(Usage);
      UsageMock
        .expects('find').withArgs({chatId: 'stubChatId0'})
        .chain('sort').withArgs('-dateCreated')
        .chain('limit').withArgs(1)
        .chain('exec')
        .yields(null, [{
          chatId: 'stubChatId0',
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
      var sinonCountStub = sinon.stub(Message, 'count', function (err, callback) {
        callback(null, 100);
      });
      var sinonAggregateStub = sinon.stub(Object.getPrototypeOf(Message), 'aggregate', function (query) {
        var stubArray = [];
        for (var i = 0, l = 20; i < l; i++) {
          var stub = {
            "_id": "stubChatId" + i,
            "title": "stubChatTitle" + i,
            "count": 100 - i
          };
          stubArray.push(stub);
        }
        return {
          sort: function () {
            return {
              limit: function () {
                return {
                  exec: function (callback) {
                    callback(null, stubArray);
                  }
                }
              }
            }
          }
        };
      });
      PremierLeagueController.getTable({
        chat: {
          id: 'stubChatId0',
          title: 'stubChatTitle0'
        },
        from: {
          id: 'stubFromId',
          username: 'stubUsername',
          first_name: 'stubFirstName',
          last_name: 'stubLastName'
        }
      }).then(function onSuccess(message) {
        (message.indexOf(Constants.PREMIER_LEAGUE.TABLE_TITLE) >= 0).should.equal(true);
        (message.indexOf('CHAMPION!!!') >= 0).should.equal(true);
      }).catch(function (err) {
        false.should.equal(true); // should fail
      }).then(function always() {
        UsageMock.verify();
        UsageMock.restore();
        sinonCountStub.restore();
        sinonAggregateStub.restore();
        findOneAndUpdateSinonStub.restore();
        usageSavesinonStub.restore();
        done();
      });
    });

    it('can get league table within 7 days for the first group with only 1 group', function (done) {
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
      var sinonCountStub = sinon.stub(Message, 'count', function (err, callback) {
        callback(null, 100);
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
                      "count": 100
                    }]);
                  }
                }
              }
            }
          }
        };
      });
      PremierLeagueController.getTable({
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
      }).then(function onSuccess(message) {
        (message.indexOf(Constants.PREMIER_LEAGUE.TABLE_TITLE) >= 0).should.equal(true);
        (message.indexOf('CHAMPION!!!') >= 0).should.equal(false);
      }).catch(function (err) {
        false.should.equal(true); // should fail
      }).then(function always() {
        UsageMock.verify();
        UsageMock.restore();
        sinonCountStub.restore();
        sinonAggregateStub.restore();
        findOneAndUpdateSinonStub.restore();
        usageSavesinonStub.restore();
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
      var sinonCountStub = sinon.stub(Message, 'count', function (err, callback) {
        callback(new Error('countError'));
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
        false.should.equal(true); // should fail
      }, function onFailure(err) {
        err.message.should.equal('countError');
      }).then(function always() {
        UsageMock.verify();
        UsageMock.restore();
        sinonCountStub.restore();
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