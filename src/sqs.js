import AWS from 'aws-sdk'
import Pino from 'pino'
import moment from 'moment'
import Statistics from './statistics'
import Help from './help'

const ACTION_KEY_ALLJUNG = 'alljung'
const ACTION_KEY_JUNGHELP = 'junghelp'
const ACTION_KEY_OFF_FROM_WORK = 'offFromWork'
const ACTION_KEY_TOPDIVER = 'topdiver'
const ACTION_KEY_TOPTEN = 'topten'

export default class SQS {
  constructor () {
    this.logger = new Pino({ level: process.env.LOG_LEVEL })
    this.sqs = new AWS.SQS()
    this.statistics = new Statistics()
    this.help = new Help()
  }

  async onEvent (event) {
    this.logger.info(`SQS onEvent start at ${moment().utcOffset(8).format()}`)
    const record = event.Records[0]
    const message = record.messageAttributes
    const chatId = Number(message.chatId.stringValue)
    const action = message.action.stringValue
    switch (action) {
      case ACTION_KEY_ALLJUNG:
        this.logger.info(`SQS onEvent alljung start at ${moment().utcOffset(8).format()}`)
        await this.statistics.allJung(chatId)
        break
      case ACTION_KEY_JUNGHELP:
        this.logger.info(`SQS onEvent junghelp start at ${moment().utcOffset(8).format()}`)
        await this.help.sendHelpMessage(chatId)
        break
      case ACTION_KEY_OFF_FROM_WORK:
        this.logger.info(`SQS onEvent offFromWork start at ${moment().utcOffset(8).format()}`)
        await this.statistics.offFromWork(chatId)
        break
      case ACTION_KEY_TOPDIVER:
        this.logger.info(`SQS onEvent topdiver start at ${moment().utcOffset(8).format()}`)
        await this.statistics.topDiver(chatId)
        break
      case ACTION_KEY_TOPTEN:
        this.logger.info(`SQS onEvent topten start at ${moment().utcOffset(8).format()}`)
        await this.statistics.topTen(chatId)
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

  async sendJungHelpMessage (message) {
    this.logger.info(`SQS sendJungHelpMessage start at ${moment().utcOffset(8).format()}`)
    return this.sqs.sendMessage({
      MessageAttributes: {
        chatId: {
          DataType: 'Number',
          StringValue: message.chat.id.toString()
        },
        action: {
          DataType: 'String',
          StringValue: ACTION_KEY_JUNGHELP
        }
      },
      MessageBody: 'sendJungHelpMessage',
      QueueUrl: process.env.EVENT_QUEUE_URL
    }).promise()
  }

  async sendTopTenMessage (message) {
    this.logger.info(`SQS sendTopTenMessage start at ${moment().utcOffset(8).format()}`)
    return this.sqs.sendMessage({
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
    }).promise()
  }

  async sendTopDiverMessage (message) {
    this.logger.info(`SQS sendTopDiverMessage start at ${moment().utcOffset(8).format()}`)
    return this.sqs.sendMessage({
      MessageAttributes: {
        chatId: {
          DataType: 'Number',
          StringValue: message.chat.id.toString()
        },
        action: {
          DataType: 'String',
          StringValue: ACTION_KEY_TOPDIVER
        }
      },
      MessageBody: 'sendTopDiverMessage',
      QueueUrl: process.env.EVENT_QUEUE_URL
    }).promise()
  }

  async sendOffFromWorkMessage (chatId) {
    this.logger.info(`SQS sendOffFromWorkMessage start at ${moment().utcOffset(8).format()}`)
    return this.sqs.sendMessage({
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
    }).promise()
  }

  async sendAllJungMessage (message) {
    this.logger.info(`SQS sendAllJungMessage start at ${moment().utcOffset(8).format()}`)
    return this.sqs.sendMessage({
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
    }).promise()
  }
}
