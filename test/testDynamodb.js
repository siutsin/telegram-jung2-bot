import test from 'ava'
import path from 'path'
import dotenv from 'dotenv'
import AWS from 'aws-sdk-mock'
import DynamoDB from '../src/dynamodb'
import stubTelegramNewMessage from './stub/telegramNewMessage'
import stubTelegramNewMessageOptional from './stub/telegramNewMessageOptional'

dotenv.config({ path: path.resolve(__dirname, '.env.testing') })

const stubPutMessage = 'successfully put item into the database'
const stubQueryMessage = { Items: 'successfully query items from the database' }

test.beforeEach(() => {
  AWS.mock('DynamoDB.DocumentClient', 'update', (params, callback) => {
    callback(null, stubPutMessage)
  })
  AWS.mock('DynamoDB.DocumentClient', 'query', (params, callback) => {
    callback(null, stubQueryMessage)
  })
  AWS.mock('DynamoDB.DocumentClient', 'scan', (params, callback) => {
    callback(null, stubQueryMessage)
  })
})

test.afterEach.always(() => {
  AWS.restore()
})

test('buildExpression', t => {
  const dynamodb = new DynamoDB()
  const message = stubTelegramNewMessage.message
  const {
    Key,
    UpdateExpression,
    ExpressionAttributeNames,
    ExpressionAttributeValues
  } = dynamodb.buildExpression({ message })
  t.is(Key.chatId, -4)
  t.regex(Key.dateCreated, /[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}\+[0-9]{2}:[0-9]{2}/)
  t.is(UpdateExpression, 'SET #chatTitle = :chatTitle, #userId = :userId, #username = :username, #firstName = :firstName, #lastName = :lastName, #ttl = :ttl')
  t.deepEqual(ExpressionAttributeNames, {
    '#chatTitle': 'chatTitle',
    '#firstName': 'firstName',
    '#lastName': 'lastName',
    '#ttl': 'ttl',
    '#userId': 'userId',
    '#username': 'username'
  })
  t.regex(ExpressionAttributeValues[':ttl'].toString(), /[0-9]{10}/)
  delete ExpressionAttributeValues[':ttl']
  t.deepEqual(ExpressionAttributeValues, {
    ':chatTitle': 'title',
    ':firstName': 'first_name',
    ':lastName': 'last_name',
    ':userId': 3,
    ':username': 'username'
  })
})

test('saveMessage', async t => {
  const dynamodb = new DynamoDB()
  const message = stubTelegramNewMessage.message
  const { updateChatIdResponse, saveStatMessageResponse } = await dynamodb.saveMessage({ message })
  t.is(updateChatIdResponse, 'successfully put item into the database')
  t.is(saveStatMessageResponse, 'successfully put item into the database')
})

test('saveMessage - optional fields', async t => {
  const dynamodb = new DynamoDB()
  const message = stubTelegramNewMessageOptional.message
  const { updateChatIdResponse, saveStatMessageResponse } = await dynamodb.saveMessage({ message })
  t.is(updateChatIdResponse, 'successfully put item into the database')
  t.is(saveStatMessageResponse, 'successfully put item into the database')
})

test('getRowsByChatId', async t => {
  const dynamodb = new DynamoDB()
  const response = await dynamodb.getRowsByChatId({ chatId: 123 })
  t.is(response[0], stubQueryMessage.Items)
})

test.serial('getRowsByChatId with LastEvaluatedKey', async t => {
  AWS.restore()
  let i = 3
  const stubObject = () => {
    const obj = {
      Items: ['dummy'],
      LastEvaluatedKey: { d: 'dummy' }
    }
    if (i <= 0) { delete obj.LastEvaluatedKey }
    i--
    return obj
  }
  AWS.mock('DynamoDB.DocumentClient', 'query', (params, callback) => {
    const obj = stubObject()
    callback(null, obj)
  })
  const dynamodb = new DynamoDB()
  const response = await dynamodb.getRowsByChatId({ chatId: 123 })
  t.is(response[0], 'dummy')
})

test.serial('getAllGroupIds with LastEvaluatedKey', async t => {
  AWS.restore()
  let i = 3
  const stubObject = () => {
    const obj = {
      Items: ['dummy'],
      LastEvaluatedKey: { d: 'dummy' }
    }
    if (i <= 0) { delete obj.LastEvaluatedKey }
    i--
    return obj
  }
  AWS.mock('DynamoDB.DocumentClient', 'scan', (params, callback) => {
    const obj = stubObject()
    callback(null, obj)
  })
  const dynamodb = new DynamoDB()
  const response = await dynamodb.getAllGroupIds()
  t.is(response[0], 'dummy')
})

test('In serverless-offline environment', async t => {
  const cache = process.env.IS_OFFLINE
  process.env.IS_OFFLINE = true
  const dynamodb = new DynamoDB()
  const message = stubTelegramNewMessage.message
  const { updateChatIdResponse, saveStatMessageResponse } = await dynamodb.saveMessage({ message })
  t.is(updateChatIdResponse, 'successfully put item into the database')
  t.is(saveStatMessageResponse, 'successfully put item into the database')
  process.env.IS_OFFLINE = cache
})
