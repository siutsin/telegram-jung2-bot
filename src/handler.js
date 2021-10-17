const Messages = require('./messages')
const OffFromWork = require('./offFromWork')
const SQS = require('./sqs')
const DynamoDB = require('./dynamodb')

const messages = new Messages()
const offFromWork = new OffFromWork()
const sqs = new SQS()
const dynamoDB = new DynamoDB()

module.exports = {
  onOffFromWork: async () => offFromWork.off(),
  onMessage: async (event) => messages.newMessage(event),
  onEvent: async (event) => sqs.onEvent(event),
  onScaleUp: async () => dynamoDB.scaleUp()
}
