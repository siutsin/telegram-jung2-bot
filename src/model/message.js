import mongoose from 'mongoose'

const MessageSchema = new mongoose.Schema({
  chatId: String,
  chatTitle: String,
  userId: String,
  username: String,
  firstName: String,
  lastName: String,
  dateCreated: {
    type: Date,
    default: Date.now
  }
})

MessageSchema.index({
  chatId: 1,
  userId: 1
})

MessageSchema.statics.getSchema = () => MessageSchema

export default mongoose.model('Message', MessageSchema)
