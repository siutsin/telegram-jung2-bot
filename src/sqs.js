const moment = require('moment')
const AWS = require('aws-sdk')
const Pino = require('pino')
const Statistics = require('./statistics')
const Settings = require('./settings')
const Help = require('./help')
const OffFromWork = require('./offFromWork')
const Bottleneck = require('bottleneck')

const ACTION_KEY_ALLJUNG = 'alljung'
const ACTION_KEY_JUNGHELP = 'junghelp'
const ACTION_KEY_OFF_FROM_WORK = 'offFromWork'
const ACTION_KEY_TOPDIVER = 'topdiver'
const ACTION_KEY_TOPTEN = 'topten'
// Admin Only
const ACTION_KEY_ENABLE_ALLJUNG = 'enableAllJung'
const ACTION_KEY_DISABLE_ALLJUNG = 'disableAllJung'
const ACTION_KEY_SET_OFF_FROM_WORK_TIME_UTC = 'setOffFromWorkTimeUTC'
const ACTION_KEY_ON_OFF_FROM_WORK = 'onOffFromWork'

// In ECS SQS polling, the key is `StringValue` instead of `stringValue`.
// This function will extract either the Lambda event key or SQS polling key.
const getStringValue = (obj) => {
  return obj?.stringValue || obj?.StringValue
}

class SQS {
  constructor () {
    this.logger = new Pino({ level: process.env.LOG_LEVEL })
    this.sqs = new AWS.SQS()
    this.statistics = new Statistics()
    this.settings = new Settings()
    this.offFromWork = new OffFromWork()
    this.help = new Help()
  }

  async onEvent (event) {
    this.logger.info(`SQS onEvent start at ${moment().format()}`)
    this.logger.debug('event')
    this.logger.debug(event)
    let record
    try {
      record = event.Records[0]
      const message = record.messageAttributes
      const action = getStringValue(message.action)
      switch (action) {
        case ACTION_KEY_ALLJUNG:
          this.logger.info(`SQS onEvent alljung start at ${moment().format()}`)
          await this.statistics.allJung({
            chatId: Number(getStringValue(message.chatId))
          })
          break
        case ACTION_KEY_JUNGHELP:
          this.logger.info(`SQS onEvent junghelp start at ${moment().format()}`)
          await this.help.sendHelpMessage({
            chatId: Number(getStringValue(message.chatId)),
            chatTitle: getStringValue(message.chatTitle)
          })
          break
        case ACTION_KEY_OFF_FROM_WORK:
          this.logger.info(`SQS onEvent offFromWork start at ${moment().format()}`)
          await this.statistics.offFromWork({
            chatId: Number(getStringValue(message.chatId))
          })
          break
        case ACTION_KEY_TOPDIVER:
          this.logger.info(`SQS onEvent topdiver start at ${moment().format()}`)
          await this.statistics.topDiver({
            chatId: Number(getStringValue(message.chatId))
          })
          break
        case ACTION_KEY_TOPTEN:
          this.logger.info(`SQS onEvent topten start at ${moment().format()}`)
          await this.statistics.topTen({
            chatId: Number(getStringValue(message.chatId))
          })
          break
        case ACTION_KEY_ENABLE_ALLJUNG:
          this.logger.info(`SQS onEvent enableAllJung start at ${moment().format()}`)
          await this.settings.enableAllJung({
            chatId: Number(getStringValue(message.chatId)),
            chatTitle: getStringValue(message.chatTitle),
            userId: Number(getStringValue(message.userId))
          })
          break
        case ACTION_KEY_DISABLE_ALLJUNG:
          this.logger.info(`SQS onEvent disableAllJung start at ${moment().format()}`)
          await this.settings.disableAllJung({
            chatId: Number(getStringValue(message.chatId)),
            chatTitle: getStringValue(message.chatTitle),
            userId: Number(getStringValue(message.userId))
          })
          break
        case ACTION_KEY_SET_OFF_FROM_WORK_TIME_UTC:
          this.logger.info(`SQS onEvent setOffFromWorkTimeUTC start at ${moment().format()}`)
          await this.settings.setOffFromWorkTimeUTC({
            chatId: Number(getStringValue(message.chatId)),
            chatTitle: getStringValue(message.chatTitle),
            userId: Number(getStringValue(message.userId)),
            offTime: getStringValue(message.offTime),
            workday: getStringValue(message.workday)
          })
          break
        case ACTION_KEY_ON_OFF_FROM_WORK:
          this.logger.info(`SQS onEvent onOffFromWork start at ${moment().format()}`)
          await this.offFromWorkStatsPerGroup(getStringValue(message.timeString))
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
    this.logger.info(`SQS onEvent end at ${moment().format()}`)
    return p
  }

  async sendJungHelpMessage (message) {
    this.logger.info(`SQS sendJungHelpMessage start at ${moment().format()}`)
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
    this.logger.info(`SQS sendTopTenMessage start at ${moment().format()}`)
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
    this.logger.info(`SQS sendTopDiverMessage start at ${moment().format()}`)
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
    this.logger.info(`SQS sendOffFromWorkMessage start at ${moment().format()}`)
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
    this.logger.info(`SQS sendAllJungMessage start at ${moment().format()}`)
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
    this.logger.info(`SQS sendEnableAllJungMessage start at ${moment().format()}`)
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
    this.logger.info(`SQS sendDisableAllJungMessage start at ${moment().format()}`)
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
    this.logger.info(`SQS sendSetOffFromWorkTimeUTC start at ${moment().format()}`)
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

  async sendOnOffFromWork (timeString) {
    this.logger.info(`SQS sendOnOffFromWork start at ${moment().format()}`)
    return this.sqs.sendMessage({
      MessageAttributes: {
        timeString: {
          DataType: 'String',
          StringValue: timeString
        },
        action: {
          DataType: 'String',
          StringValue: ACTION_KEY_ON_OFF_FROM_WORK
        }
      },
      MessageBody: 'sendOnOffFromWork',
      QueueUrl: process.env.EVENT_QUEUE_URL
    }).promise()
  }

  // Urgent fix, these functions shouldn't be here, but there is a circular dependency issue.
  // https://github.com/siutsin/telegram-jung2-bot/issues/1884

  async offFromWorkStatsPerGroup (timeString) {
    this.logger.info(`offFromWorkStatsPerGroup start at ${moment().format()}`)
    const chatIds = await this.offFromWork.getOffChatIds(timeString)
    const limiter = new Bottleneck({ // 200 per second
      maxConcurrent: 1,
      minTime: 5
    })
    this.logger.debug('chatIds:', chatIds)
    for (const chatId of chatIds) {
      this.logger.info(`chatId: ${chatId}`)
      await limiter.schedule(() => this.sendOffFromWorkMessage(chatId))
    }
    this.logger.info(`offFromWorkStatsPerGroup finish at ${moment().format()}`)
  }
}

module.exports = SQS
