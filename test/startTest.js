require('codecov');
var log = require('log-to-file-and-console-node');
log.removeConsole();
process.env.MESSAGE_CONTROLLER = 'mongoMessage';
// express
// TODO: add test case for app.js
// route
require('./route/testBotHandler');
// TODO: add test case for route/root.js
// model
require('./model/testMessage');
require('./model/testUsage');
// controller
require('./controller/testMongoMessage');
require('./controller/testUsage');
require('./controller/testMessageFacade');