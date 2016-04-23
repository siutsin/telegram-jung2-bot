var log = require('log-to-file-and-console-node');
log.removeConsole();
// route
require('./route/testBotHandler');
// TODO: add test case for route/root
// model
require('./model/testMessage');
require('./model/testUsage');
// controller
require('./controller/testMessage');
require('./controller/testUsage');