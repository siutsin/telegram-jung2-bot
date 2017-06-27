import mongoose from 'mongoose'

const UsageSchema = new mongoose.Schema({
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

UsageSchema.statics.getSchema = () => UsageSchema

export default mongoose.model('UsageClass', UsageSchema)
