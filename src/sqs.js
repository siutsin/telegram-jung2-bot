import AWS from 'aws-sdk'
import Statistics from './statistics'

const ACTION_KEY_TOPTEN = 'topten'
const ACTION_KEY_ALLJUNG = 'alljung'

export default class SQS {
  constructor () {
    this.sqs = new AWS.SQS()
    this.statistics = new Statistics()
  }

  async onEvent (event) {
    const record = event.Records[0]
    const message = record.messageAttributes
    const chatId = Number(message.chatId.stringValue)
    const action = message.action.stringValue
    if (action === ACTION_KEY_ALLJUNG) {
      await this.statistics.allJung(chatId)
    } else if (action === ACTION_KEY_TOPTEN) {
      await this.statistics.topTen(chatId)
    }
    const deleteParams = {
      QueueUrl: process.env.EVENT_QUEUE_URL,
      ReceiptHandle: record.receiptHandle
    }
    return this.sqs.deleteMessage(deleteParams).promise()
  }

  async sendSQSMessage (sqsParams) {
    return this.sqs.sendMessage(sqsParams).promise()
  }

  async sendTopTenMessage (message) {
    return this.sendSQSMessage({
      MessageAttributes: {
        chatId: {
          DataType: 'Number',
          StringValue: message.chat.id.toString()
        },
        action: {
          DataType: 'String',
          StringValue: ACTION_KEY_TOPTEN
        }
      },
      MessageBody: 'sendTopTenMessage',
      QueueUrl: process.env.EVENT_QUEUE_URL
    })
  }

  async sendAllJungMessage (message) {
    return this.sendSQSMessage({
      MessageAttributes: {
        chatId: {
          DataType: 'Number',
          StringValue: message.chat.id.toString()
        },
        action: {
          DataType: 'String',
          StringValue: ACTION_KEY_ALLJUNG
        }
      },
      MessageBody: 'sendAllJungMessage',
      QueueUrl: process.env.EVENT_QUEUE_URL
    })
  }
}
