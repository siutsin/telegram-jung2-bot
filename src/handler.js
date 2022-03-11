const Messages = require('./messages')
const SQS = require('./sqs')
const DynamoDB = require('./dynamodb')

// should make it DI
const messages = new Messages()
const sqs = new SQS()
const dynamoDB = new DynamoDB()

module.exports = {
  onOffFromWork: async (timeString) => sqs.sendOnOffFromWork(timeString),
  onMessage: async (event) => messages.newMessage(event),
  onEvent: async (event) => sqs.onEvent(event),
  onScaleUp: async () => dynamoDB.scaleUp()
}
