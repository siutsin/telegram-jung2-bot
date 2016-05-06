require('dotenv').load();

global.cachedLastSender = {
  //chatId: 'userId'
};
var ds = require('./' + process.env.DATASTORE);
module.exports = ds;
ds.init();