import AWS from 'aws-sdk'
import Pino from 'pino'
import moment from 'moment'
import Statistics from './statistics'

const ACTION_KEY_TOPTEN = 'topten'
const ACTION_KEY_ALLJUNG = 'alljung'
const ACTION_KEY_OFF_FROM_WORK = 'offFromWork'

export default class SQS {
  constructor () {
    this.logger = new Pino({ level: process.env.LOG_LEVEL })
    this.sqs = new AWS.SQS()
    this.statistics = new Statistics()
  }

  async onEvent (event) {
    this.logger.info(`SQS onEvent start at ${moment().utcOffset(8).format()}`)
    const record = event.Records[0]
    const message = record.messageAttributes
    const chatId = Number(message.chatId.stringValue)
    const action = message.action.stringValue
    switch (action) {
      case ACTION_KEY_ALLJUNG:
        await this.statistics.allJung(chatId)
        break
      case ACTION_KEY_TOPTEN:
        await this.statistics.topTen(chatId)
        break
      case ACTION_KEY_OFF_FROM_WORK:
        await this.statistics.offFromWork(chatId)
        break
    }
    const deleteParams = {
      QueueUrl: process.env.EVENT_QUEUE_URL,
      ReceiptHandle: record.receiptHandle
    }
    const p = this.sqs.deleteMessage(deleteParams).promise()
    this.logger.info(`SQS onEvent end at ${moment().utcOffset(8).format()}`)
    return p
  }

  async sendSQSMessage (sqsParams) {
    this.logger.info(`SQS sendSQSMessage start at ${moment().utcOffset(8).format()}`)
    return this.sqs.sendMessage(sqsParams).promise()
  }

  async sendTopTenMessage (message) {
    this.logger.info(`SQS sendTopTenMessage start at ${moment().utcOffset(8).format()}`)
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

  async sendOffFromWorkMessage (chatId) {
    this.logger.info(`SQS sendOffFromWorkMessage start at ${moment().utcOffset(8).format()}`)
    return this.sendSQSMessage({
      MessageAttributes: {
        chatId: {
          DataType: 'Number',
          StringValue: chatId.toString()
        },
        action: {
          DataType: 'String',
          StringValue: ACTION_KEY_OFF_FROM_WORK
        }
      },
      MessageBody: 'sendOffFromWorkMessage',
      QueueUrl: process.env.EVENT_QUEUE_URL
    })
  }

  async sendAllJungMessage (message) {
    this.logger.info(`SQS sendAllJungMessage start at ${moment().utcOffset(8).format()}`)
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
