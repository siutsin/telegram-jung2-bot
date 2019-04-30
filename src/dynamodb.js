import moment from 'moment'
import AWS from 'aws-sdk'
import Pino from 'pino'
import uuid from 'uuid'

export default class DynamoDB {
  constructor (options) {
    // TODO: remove local testing code
    if (process.env.IS_OFFLINE) {
      options = {
        region: 'localhost',
        endpoint: 'http://localhost:8000',
        accessKeyId: 'DEFAULT_ACCESS_KEY',
        secretAccessKey: 'DEFAULT_SECRET'
      }
    }
    this.documentClient = new AWS.DynamoDB.DocumentClient(options)
    this.logger = new Pino({ level: process.env.LOG_LEVEL })
  }

  async saveChatId ({ message, days = 7 }) {
    const params = {
      TableName: process.env.CHATID_TABLE,
      Key: { chatId: message.chat.id },
      UpdateExpression: 'set #dateCreated = :dateCreated and set #ttl = :ttl',
      ExpressionAttributeNames: {
        '#dateCreated': 'dateCreated',
        '#ttl': 'ttl'
      },
      ExpressionAttributeValues: {
        ':dateCreated': moment().utcOffset(8).format(),
        ':ttl': moment().utcOffset(8).add(days, 'days').unix()
      }
    }
    this.logger.debug('params', params)
    const response = await this.documentClient.update(params).promise()
    this.logger.trace('response', response)
    return response
  }

  async saveStatMessage ({ message, days = 7 }) {
    const item = {
      id: uuid.v4(),
      chatId: message.chat.id,
      chatTitle: message.chat.title,
      userId: message.from.id,
      username: message.from.username,
      firstName: message.from.first_name,
      lastName: message.from.last_name,
      dateCreated: moment().utcOffset(8).format(),
      ttl: moment().utcOffset(8).add(days, 'days').unix()
    }
    this.logger.debug('item', item)
    const response = await this.documentClient.put({ TableName: process.env.MESSAGE_TABLE, Item: item }).promise()
    this.logger.trace('response', response)
    return response
  }

  async saveMessage (options) {
    const saveChatIdPromise = this.saveChatId(options)
    const saveStatMessagePromise = this.saveStatMessage(options)
    const promises = [saveChatIdPromise, saveStatMessagePromise]
    const [saveChatIdResponse, saveStatMessageResponse] = await Promise.all(promises)
    return { saveChatIdResponse, saveStatMessageResponse }
  }

  async getRowsByChatId ({ chatId, days = 7 }) {
    const params = {
      TableName: process.env.MESSAGE_TABLE,
      IndexName: process.env.MESSAGE_TABLE_GSI,
      KeyConditionExpression: 'chatId = :chat_id AND dateCreated > :date_created',
      ScanIndexForward: false,
      ExpressionAttributeValues: {
        ':chat_id': chatId,
        ':date_created': moment().utcOffset(8).subtract(days, 'days').format()
      }
    }
    const result = await this.documentClient.query(params).promise()
    this.logger.trace(result.Items)
    return result.Items
  }

  async getAllRowsWithinDays ({ days = 7 } = {}) {
    const params = {
      TableName: process.env.MESSAGE_TABLE,
      ScanIndexForward: false,
      FilterExpression: 'dateCreated > :date_created',
      ExpressionAttributeValues: {
        ':date_created': moment().utcOffset(8).subtract(days, 'days').format()
      }
    }
    const result = await this.documentClient.scan(params).promise()
    this.logger.trace(result.Items)
    return result.Items
  }
}
