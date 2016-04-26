'use strict';

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

MessageSchema.index({
  chatId: 1,
  userId: 1
});

module.exports = mongoose.model('Message', MessageSchema);
