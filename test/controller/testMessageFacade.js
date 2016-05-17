'use strict';

var sinon = require('sinon');
var Mongoose = require('mongoose');
var MessageController = require('../../controller/messageFacade');

describe('MessageFacade', function () {

  describe('init', function() {
    it('test init', function (done) {
      var mongooseStub = sinon.stub(Mongoose, 'connect', function () {});
      MessageController.init();
      mongooseStub.restore();
      done();
    });
    it('test skip init', function (done) {
      MessageController.init(true);
      done();
    });
  });

});