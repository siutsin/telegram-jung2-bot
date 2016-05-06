require('codecov');
var log = require('log-to-file-and-console-node');
log.removeConsole();

require('dotenv').load();

// Since controller/testMessage is coupled with mongoose, only the 'mongo'
// data store will pass the test.
process.env.DATASTORE = 'mongo';

// datastore, must be executed first to stub mongoose.connect(...)
require('./ds/testMongo');

// express
// TODO: add test case for app.js
// route
require('./route/testBotHandler');
// TODO: add test case for route/root.js
// model
require('./model/testMessage');
require('./model/testUsage');
// controller
require('./controller/testMessage');
require('./controller/testUsage');