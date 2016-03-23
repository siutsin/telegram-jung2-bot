var mongoose = require('mongoose');

var MessageSchema = new mongoose.Schema({
  chatId: String,
  userId: String,
  username: String,
  firstName: String,
  lastName: String,
  dateCreated: {
    type: Date,
    default: Date.now
  }
});

module.exports = mongoose.model('Message', MessageSchema);
