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

  async saveMessage (message) {
    const item = {
      id: uuid.v4(),
      chatId: message.chat.id,
      chatTitle: message.chat.title,
      userId: message.from.id,
      username: message.from.username,
      firstName: message.from.first_name,
      lastName: message.from.last_name,
      dateCreated: moment().utcOffset(8).format(),
      ttl: moment().utcOffset(8).add(7, 'days').format()
    }
    this.logger.debug('item', item)
    const response = await this.documentClient.put({ TableName: process.env.MESSAGE_TABLE, Item: item }).promise()
    this.logger.debug('response', response)
    return response
  }

  async getRowsByChatId (chatId) {
    const params = {
      TableName: process.env.MESSAGE_TABLE,
      IndexName: process.env.MESSAGE_TABLE_GSI,
      KeyConditionExpression: 'chatId = :chat_id AND dateCreated > :date_created',
      ScanIndexForward: false,
      ExpressionAttributeValues: {
        ':chat_id': chatId,
        ':date_created': moment().utcOffset(8).subtract(7, 'days').format()
      }
    }
    const result = await this.documentClient.query(params).promise()
    this.logger.debug(result.Items)
    return result.Items
  }
}
