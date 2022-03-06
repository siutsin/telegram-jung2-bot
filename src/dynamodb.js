const moment = require('moment')
const AWS = require('aws-sdk')
const Pino = require('pino')
const WorkdayHelper = require('./workdayHelper')

const LEGACY_OFF_JOB_WEEKDAY = new Set(['MON', 'TUE', 'WED', 'THU', 'FRI'])

class DynamoDB {
  constructor (options) {
    this.logger = new Pino({ level: process.env.LOG_LEVEL })
    this.logger.trace(`dynamodb.js::constructor options: ${JSON.stringify(options)}`)
    this.dynamoDB = new AWS.DynamoDB(options)
    this.logger.trace('dynamodb.js::constructor this.dynamoDB:')
    this.logger.trace(this.dynamoDB)
    this.documentClient = new AWS.DynamoDB.DocumentClient(options)
    this.workdayHelper = new WorkdayHelper()
    this.logger.trace('dynamodb.js::constructor this.documentClient:')
    this.logger.trace(this.documentClient)
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
    this.logger.debug(`dynamodb.js::saveStats params: ${JSON.stringify(params)}`)
    const response = await this.documentClient.update(params).promise()
    this.logger.trace(`dynamodb.js::saveStats response: ${JSON.stringify(response)}`)
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
    this.logger.debug(`dynamodb.js::updateChatId params: ${JSON.stringify(params)}`)
    const response = await this.documentClient.update(params).promise()
    this.logger.trace(`dynamodb.js::updateChatId response: ${JSON.stringify(response)}`)
    return response
  }

  async setOffFromWorkTimeUTC ({ chatId, offTime, workday }) {
    const params = {
      TableName: process.env.CHATID_TABLE,
      Key: { chatId: chatId },
      UpdateExpression: 'SET #ot = :ot, #wd = :wd',
      ExpressionAttributeNames: {
        '#ot': 'offTime',
        '#wd': 'workday'
      },
      ExpressionAttributeValues: {
        ':ot': offTime,
        ':wd': this.workdayHelper.workdayStringToBinary(workday)
      }
    }
    this.logger.debug(`dynamodb.js::setOffFromWorkTimeUTC params: ${JSON.stringify(params)}`)
    const response = await this.documentClient.update(params).promise()
    this.logger.trace(`dynamodb.js::setOffFromWorkTimeUTC response: ${JSON.stringify(response)}`)
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
    this.logger.debug(`dynamodb.js::enableAllJung params: ${JSON.stringify(params)}`)
    const response = await this.documentClient.update(params).promise()
    this.logger.trace(`dynamodb.js::enableAllJung response: ${JSON.stringify(response)}`)
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
    this.logger.debug(`dynamodb.js::disableAllJung params: ${JSON.stringify(params)}`)
    const response = await this.documentClient.update(params).promise()
    this.logger.trace(`dynamodb.js::disableAllJung response: ${JSON.stringify(response)}`)
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
    this.logger.debug(`dynamodb.js::updateChatIdMessagesCount params: ${JSON.stringify(params)}`)
    const response = await this.documentClient.update(params).promise()
    this.logger.trace(`dynamodb.js::updateChatIdMessagesCount response: ${JSON.stringify(response)}`)
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

  async getAllGroupIds ({ offTime, weekday }) {
    const _getAllGroupIds = async (startKey) => {
      let filterExpression = '#ot = :ot'
      const expressionAttributeNames = {
        '#ot': 'offTime'
      }
      const expressionAttributeValues = {
        ':ot': offTime
      }

      // legacy off time, HKT 1800 (UTC 1000), MON-FRI
      if (offTime === '1000' && LEGACY_OFF_JOB_WEEKDAY.has(weekday)) {
        filterExpression += ' Or (attribute_not_exists(#ot) And attribute_not_exists(#wd))'
        expressionAttributeNames['#wd'] = 'workday'
      }

      const params = {
        TableName: process.env.CHATID_TABLE,
        FilterExpression: filterExpression,
        ExpressionAttributeNames: expressionAttributeNames,
        ExpressionAttributeValues: expressionAttributeValues
      }
      if (startKey) {
        params.ExclusiveStartKey = startKey
      }
      // scan is expensive, but it is probably good enough for the database size at the moment
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
    // both undefined === default legacy off time
    return rows.filter(r => (r.workday === undefined && r.offTime === undefined) ||
      this.workdayHelper.isWeekdayMatchBinary(weekday, r.workday))
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

module.exports = DynamoDB
