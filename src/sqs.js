const moment = require('moment')
const AWS = require('aws-sdk')
const Pino = require('pino')
const Statistics = require('./statistics')
const Settings = require('./settings')
const Help = require('./help')

const ACTION_KEY_ALLJUNG = 'alljung'
const ACTION_KEY_JUNGHELP = 'junghelp'
const ACTION_KEY_OFF_FROM_WORK = 'offFromWork'
const ACTION_KEY_TOPDIVER = 'topdiver'
const ACTION_KEY_TOPTEN = 'topten'
// Admin Only
const ACTION_KEY_ENABLE_ALLJUNG = 'enableAllJung'
const ACTION_KEY_DISABLE_ALLJUNG = 'disableAllJung'
const ACTION_KEY_SET_OFF_FROM_WORK_TIME_UTC = 'setOffFromWorkTimeUTC'

// In ECS SQS polling, the key is `StringValue` instead of `stringValue`.
// This function will extract either the Lambda event key or SQS polling key.
const getStringValue = (obj) => {
  return obj.stringValue || obj.StringValue
}

class SQS {
  constructor () {
    this.logger = new Pino({ level: process.env.LOG_LEVEL })
    this.sqs = new AWS.SQS()
    this.statistics = new Statistics()
    this.settings = new Settings()
    this.help = new Help()
  }

  async onEvent (event) {
    this.logger.info(`SQS onEvent start at ${moment().utcOffset(8).format()}`)
    this.logger.debug('event')
    this.logger.debug(event)
    let record
    try {
      record = event.Records[0]
      const message = record.messageAttributes
      const chatId = Number(getStringValue(message.chatId))
      const action = getStringValue(message.action)
      switch (action) {
        case ACTION_KEY_ALLJUNG:
          this.logger.info(`SQS onEvent alljung start at ${moment().utcOffset(8).format()}`)
          await this.statistics.allJung({ chatId })
          break
        case ACTION_KEY_JUNGHELP:
          this.logger.info(`SQS onEvent junghelp start at ${moment().utcOffset(8).format()}`)
          await this.help.sendHelpMessage({ chatId, chatTitle: getStringValue(message.chatTitle) })
          break
        case ACTION_KEY_OFF_FROM_WORK:
          this.logger.info(`SQS onEvent offFromWork start at ${moment().utcOffset(8).format()}`)
          await this.statistics.offFromWork({ chatId })
          break
        case ACTION_KEY_TOPDIVER:
          this.logger.info(`SQS onEvent topdiver start at ${moment().utcOffset(8).format()}`)
          await this.statistics.topDiver({ chatId })
          break
        case ACTION_KEY_TOPTEN:
          this.logger.info(`SQS onEvent topten start at ${moment().utcOffset(8).format()}`)
          await this.statistics.topTen({ chatId })
          break
        case ACTION_KEY_ENABLE_ALLJUNG:
          this.logger.info(`SQS onEvent enableAllJung start at ${moment().utcOffset(8).format()}`)
          await this.settings.enableAllJung({
            chatId,
            chatTitle: getStringValue(message.chatTitle),
            userId: Number(getStringValue(message.userId))
          })
          break
        case ACTION_KEY_DISABLE_ALLJUNG:
          this.logger.info(`SQS onEvent disableAllJung start at ${moment().utcOffset(8).format()}`)
          await this.settings.disableAllJung({
            chatId,
            chatTitle: getStringValue(message.chatTitle),
            userId: Number(getStringValue(message.userId))
          })
          break
        case ACTION_KEY_SET_OFF_FROM_WORK_TIME_UTC:
          this.logger.info(`SQS onEvent setOffFromWorkTimeUTC start at ${moment().utcOffset(8).format()}`)
          await this.settings.setOffFromWorkTimeUTC({
            chatId,
            chatTitle: getStringValue(message.chatTitle),
            userId: Number(getStringValue(message.userId)),
            offTime: getStringValue(message.offTime),
            workday: getStringValue(message.workday)
          })
          break
      }
    } catch (e) {
      this.logger.error('onEvent error')
      this.logger.error(e)
      this.logger.error('onEvent error sqs event.Records')
      this.logger.error(event.Records)
      return e.message
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
        chatTitle: {
          DataType: 'String',
          StringValue: message.chat.title
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

  async sendEnableAllJungMessage (message) {
    this.logger.info(`SQS sendEnableAllJungMessage start at ${moment().utcOffset(8).format()}`)
    return this.sqs.sendMessage({
      MessageAttributes: {
        chatId: {
          DataType: 'Number',
          StringValue: message.chat.id.toString()
        },
        chatTitle: {
          DataType: 'String',
          StringValue: message.chat.title
        },
        userId: {
          DataType: 'Number',
          StringValue: message.from.id.toString()
        },
        action: {
          DataType: 'String',
          StringValue: ACTION_KEY_ENABLE_ALLJUNG
        }
      },
      MessageBody: 'sendEnableAllJungMessage',
      QueueUrl: process.env.EVENT_QUEUE_URL
    }).promise()
  }

  async sendDisableAllJungMessage (message) {
    this.logger.info(`SQS sendDisableAllJungMessage start at ${moment().utcOffset(8).format()}`)
    return this.sqs.sendMessage({
      MessageAttributes: {
        chatId: {
          DataType: 'Number',
          StringValue: message.chat.id.toString()
        },
        chatTitle: {
          DataType: 'String',
          StringValue: message.chat.title
        },
        userId: {
          DataType: 'Number',
          StringValue: message.from.id.toString()
        },
        action: {
          DataType: 'String',
          StringValue: ACTION_KEY_DISABLE_ALLJUNG
        }
      },
      MessageBody: 'sendDisableAllJungMessage',
      QueueUrl: process.env.EVENT_QUEUE_URL
    }).promise()
  }

  async sendSetOffFromWorkTimeUTC ({ message, offTime, workday }) {
    this.logger.info(`SQS sendSetOffFromWorkTimeUTC start at ${moment().utcOffset(8).format()}`)
    return this.sqs.sendMessage({
      MessageAttributes: {
        chatId: {
          DataType: 'Number',
          StringValue: message.chat.id.toString()
        },
        chatTitle: {
          DataType: 'String',
          StringValue: message.chat.title
        },
        userId: {
          DataType: 'Number',
          StringValue: message.from.id.toString()
        },
        offTime: {
          DataType: 'String',
          StringValue: offTime
        },
        workday: {
          DataType: 'String',
          StringValue: workday
        },
        action: {
          DataType: 'String',
          StringValue: ACTION_KEY_SET_OFF_FROM_WORK_TIME_UTC
        }
      },
      MessageBody: 'sendSetOffFromWorkTimeUTC',
      QueueUrl: process.env.EVENT_QUEUE_URL
    }).promise()
  }
}

module.exports = SQS
