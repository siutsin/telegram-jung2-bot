import moment from 'moment'
import AWS from 'aws-sdk'
import Pino from 'pino'

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
      UpdateExpression: 'SET #dateCreated = :dateCreated, #ttl = :ttl',
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
    const params = {
      TableName: process.env.MESSAGE_TABLE,
      Key: {
        chatId: message.chat.id,
        dateCreated: moment().utcOffset(8).format()
      },
      UpdateExpression: 'SET #ct = :ct, #ui = :ui, #un = :un, #fn = :fn, #ln = :ln, #ttl = :ttl',
      ExpressionAttributeNames: {
        '#ct': 'chatTitle',
        '#ui': 'userId',
        '#un': 'username',
        '#fn': 'firstName',
        '#ln': 'lastName',
        '#ttl': 'ttl'
      },
      ExpressionAttributeValues: {
        ':ct': message.chat.title,
        ':ui': message.from.id,
        ':un': message.from.username,
        ':fn': message.from.first_name,
        ':ln': message.from.last_name,
        ':ttl': moment().add(days, 'days').unix()
      }
    }
    this.logger.debug('params', params)
    const response = await this.documentClient.update(params).promise()
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
    const _getRowsByChatId = async (startKey) => {
      const params = {
        TableName: process.env.MESSAGE_TABLE,
        KeyConditionExpression: 'chatId = :chat_id AND dateCreated > :date_created',
        ScanIndexForward: false,
        ExpressionAttributeValues: {
          ':chat_id': chatId,
          ':date_created': moment().utcOffset(8).subtract(days, 'days').format()
        }
      }
      if (startKey) {
        params.ExclusiveStartKey = startKey
      }
      const result = await this.documentClient.query(params).promise()
      this.logger.info(`Count: ${result.Count} result.LastEvaluatedKey: ${JSON.stringify(result.LastEvaluatedKey)}`)
      this.logger.trace(result)
      return result
    }
    let lastEvaluatedKey
    let i = 0
    let rows = []
    do {
      this.logger.info(`i: ${i} lastEvaluatedKey: ${JSON.stringify(lastEvaluatedKey)}`)
      const result = await _getRowsByChatId(lastEvaluatedKey)
      rows = rows.concat(result.Items)
      lastEvaluatedKey = result.LastEvaluatedKey
      i++
    } while (lastEvaluatedKey)
    this.logger.info(`getRowsByChatId rows count: ${rows.length}`)
    return rows
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
