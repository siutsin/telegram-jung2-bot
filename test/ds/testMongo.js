'use strict';

require('chai').should();
var sinon = require('sinon');
require('sinon-mongoose');
var log = require('log-to-file-and-console-node');
var _ = require('lodash');
var mongoose = require('mongoose');

// Stub mongoose.connect
var sinonStub = sinon.stub(mongoose, 'connect', function () {
});

var ds = require('../../ds/datastore.js');

describe('MongoDAO', function () {
	it('Test init', function (done) {
		process.env.OPENSHIFT_MONGODB_DB_PASSWORD = true;
		ds.init();
		process.env.MONGODB_URL = true;
		ds.init();
		done();
	});
});