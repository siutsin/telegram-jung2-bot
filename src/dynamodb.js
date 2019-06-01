import moment from 'moment'
import * as AWS from 'aws-sdk'
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
    this.dynamoDB = new AWS.DynamoDB(options)
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

  async updateChatId ({ message, days = 7 }) {
    const params = {
      TableName: process.env.CHATID_TABLE,
      Key: { chatId: message.chat.id },
      UpdateExpression: 'SET #ct = :ct, #dc = :dc, #ttl = :ttl',
      ExpressionAttributeNames: {
        '#ct': 'chatTitle',
        '#dc': 'dateCreated',
        '#ttl': 'ttl'
      },
      ExpressionAttributeValues: {
        ':ct': message.chat.title,
        ':dc': moment().utcOffset(8).format(),
        ':ttl': moment().utcOffset(8).add(days, 'days').unix()
      }
    }
    this.logger.debug('params', params)
    const response = await this.documentClient.update(params).promise()
    this.logger.trace('response', response)
    return response
  }

  async enableAllJung ({ chatId }) {
    const params = {
      TableName: process.env.CHATID_TABLE,
      Key: { chatId },
      UpdateExpression: 'SET #eaj = :eaj',
      ExpressionAttributeNames: {
        '#eaj': 'enableAllJung'
      },
      ExpressionAttributeValues: {
        ':eaj': true
      }
    }
    this.logger.debug('params', params)
    const response = await this.documentClient.update(params).promise()
    this.logger.trace('response', response)
    return response
  }

  async disableAllJung ({ chatId }) {
    const params = {
      TableName: process.env.CHATID_TABLE,
      Key: { chatId },
      UpdateExpression: 'SET #eaj = :eaj',
      ExpressionAttributeNames: {
        '#eaj': 'enableAllJung'
      },
      ExpressionAttributeValues: {
        ':eaj': false
      }
    }
    this.logger.debug('params', params)
    const response = await this.documentClient.update(params).promise()
    this.logger.trace('response', response)
    return response
  }

  async updateChatIdMessagesCount ({ chatId, userCount, messageCount }) {
    const params = {
      TableName: process.env.CHATID_TABLE,
      Key: { chatId },
      UpdateExpression: 'SET #uc = :uc, #mc = :mc, #mpu = :mpu, #ct = :ct',
      ExpressionAttributeNames: {
        '#uc': 'userCount',
        '#mc': 'messageCount',
        '#mpu': 'messagePerUser',
        '#ct': 'countTimestamp'
      },
      ExpressionAttributeValues: {
        ':uc': userCount,
        ':mc': messageCount,
        ':mpu': messageCount / userCount,
        ':ct': moment().utcOffset(8).format()
      }
    }
    this.logger.debug('params', params)
    const response = await this.documentClient.update(params).promise()
    this.logger.trace('response', response)
    return response
  }

  async saveMessage (options) {
    const updateChatIdPromise = this.updateChatId(options)
    const saveStatMessagePromise = this.saveStats(options)
    const promises = [updateChatIdPromise, saveStatMessagePromise]
    const [updateChatIdResponse, saveStatMessageResponse] = await Promise.all(promises)
    return { updateChatIdResponse, saveStatMessageResponse }
  }

  async getStatsByChatId ({ chatId }) {
    const params = {
      TableName: process.env.CHATID_TABLE,
      KeyConditionExpression: 'chatId = :chat_id',
      ExpressionAttributeValues: {
        ':chat_id': chatId
      }
    }
    const result = await this.documentClient.query(params).promise()
    this.logger.trace(result)
    return result
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
    let lastEvaluatedKey = false
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
      this.logger.trace(result)
      return result
    }
    let lastEvaluatedKey = false
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

  async scaleUp () {
    const describeParams = {
      TableName: process.env.MESSAGE_TABLE
    }
    const describeResponse = await this.dynamoDB.describeTable(describeParams).promise()
    const WriteCapacityUnits = describeResponse.Table.ProvisionedThroughput.WriteCapacityUnits
    const readCapacityNumber = Number(process.env.SCALE_UP_READ_CAPACITY)
    const ReadCapacityUnits = readCapacityNumber || describeResponse.Table.ProvisionedThroughput.ReadCapacityUnits
    const updateParams = {
      ProvisionedThroughput: {
        ReadCapacityUnits,
        WriteCapacityUnits
      },
      TableName: process.env.MESSAGE_TABLE
    }
    try {
      const response = await this.dynamoDB.updateTable(updateParams).promise()
      this.logger.debug(response)
      return response
    } catch (e) {
      this.logger.warn(e.message)
      if (e.message.includes('Subscriber limit exceeded') ||
        e.message.includes('The provisioned throughput for the table will not change') ||
        e.message.includes('Attempt to change a resource which is still in use')) {
        return e.message
      }
      throw e
    }
  }
}
