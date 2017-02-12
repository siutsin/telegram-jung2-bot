'use strict'

var mongoose = require('mongoose')

var UsageSchema = new mongoose.Schema({
  chatId: String,
  notified: {
    type: Boolean,
    default: false
  },
  dateCreated: {
    type: Date,
    default: Date.now
  }
})

UsageSchema.index({
  chatId: 1,
  notified: 1
})

UsageSchema.statics.getSchema = function () {
  return UsageSchema
}

module.exports = mongoose.model('UsageClass', UsageSchema)
