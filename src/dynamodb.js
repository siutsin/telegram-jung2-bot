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

  buildExpression ({ message, days }) {
    const updateExpressionArray = []
    const ExpressionAttributeNames = {}
    const ExpressionAttributeValues = {}
    const Key = {
      chatId: message.chat.id,
      dateCreated: moment().utcOffset(8).format()
    }

    const mapping = {
      'chat.title': 'chatTitle',
      'from.id': 'userId',
      'from.username': 'username',
      'from.first_name': 'firstName',
      'from.last_name': 'lastName'
    }
    Object.keys(mapping).forEach((key) => {
      const attribute = mapping[key]
      const path = key.split('.')
      if (message[path[0]][path[1]]) {
        updateExpressionArray.push(`#${attribute} = :${attribute}`)
        ExpressionAttributeNames[`#${attribute}`] = attribute
        ExpressionAttributeValues[`:${attribute}`] = message[path[0]][path[1]]
      }
    })

    updateExpressionArray.push('#ttl = :ttl')
    ExpressionAttributeNames['#ttl'] = 'ttl'
    ExpressionAttributeValues[':ttl'] = moment().add(days, 'days').unix()

    const UpdateExpression = `SET ${updateExpressionArray.join(', ')}`
    return { Key, UpdateExpression, ExpressionAttributeNames, ExpressionAttributeValues }
  }

  async saveStats ({ message, days = 7 }) {
    const {
      Key,
      UpdateExpression,
      ExpressionAttributeNames,
      ExpressionAttributeValues
    } = this.buildExpression({ message, days })
    const params = {
      TableName: process.env.MESSAGE_TABLE,
      Key,
      UpdateExpression,
      ExpressionAttributeNames,
      ExpressionAttributeValues
    }
    this.logger.debug('params', params)
    const response = await this.documentClient.update(params).promise()
    this.logger.trace('response', response)
    return response
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

  async saveMessage (options) {
    const saveChatIdPromise = this.saveChatId(options)
    const saveStatMessagePromise = this.saveStats(options)
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

  async getAllGroupIds () {
    const _getAllGroupIds = async (startKey) => {
      const params = {
        TableName: process.env.CHATID_TABLE
      }
      if (startKey) {
        params.ExclusiveStartKey = startKey
      }
      const result = await this.documentClient.scan(params).promise()
      this.logger.debug(result)
      return result
    }
    let lastEvaluatedKey
    let i = 0
    let rows = []
    do {
      this.logger.info(`i: ${i} lastEvaluatedKey: ${JSON.stringify(lastEvaluatedKey)}`)
      const result = await _getAllGroupIds(lastEvaluatedKey)
      rows = rows.concat(result.Items)
      lastEvaluatedKey = result.LastEvaluatedKey
      i++
    } while (lastEvaluatedKey)
    this.logger.info(`_getAllGroupIds rows count: ${rows.length}`)
    return rows
  }
}
