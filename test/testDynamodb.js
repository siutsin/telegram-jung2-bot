import test from 'ava'
import path from 'path'
import dotenv from 'dotenv'
import AWS from 'aws-sdk-mock'
import DynamoDB from '../src/dynamodb'
import stubTelegramNewMessage from './stub/telegramNewMessage'

dotenv.config({ path: path.resolve(__dirname, '.env.testing') })

const stubPutMessage = 'successfully put item into the database'
const stubQueryMessage = { Items: 'successfully query items from the database' }

test.beforeEach(t => {
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

test.afterEach.always(t => {
  AWS.restore()
})

test('saveMessage', async t => {
  const dynamodb = new DynamoDB()
  const message = stubTelegramNewMessage.message
  const { saveChatIdResponse, saveStatMessageResponse } = await dynamodb.saveMessage({ message })
  t.is(saveChatIdResponse, 'successfully put item into the database')
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
      LastEvaluatedKey: { d: 'ummy' }
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

test('getAllRowsWithinDays - 5 days', async t => {
  const dynamodb = new DynamoDB()
  const response = await dynamodb.getAllRowsWithinDays({ days: 5 })
  t.is(response, stubQueryMessage.Items)
})

test('getAllRowsWithinDays - default 7 days', async t => {
  const dynamodb = new DynamoDB()
  const response = await dynamodb.getAllRowsWithinDays()
  t.is(response, stubQueryMessage.Items)
})

test('In serverless-offline environment', async t => {
  const cache = process.env.IS_OFFLINE
  process.env.IS_OFFLINE = true
  const dynamodb = new DynamoDB()
  const message = stubTelegramNewMessage.message
  const { saveChatIdResponse, saveStatMessageResponse } = await dynamodb.saveMessage({ message })
  t.is(saveChatIdResponse, 'successfully put item into the database')
  t.is(saveStatMessageResponse, 'successfully put item into the database')
  process.env.IS_OFFLINE = cache
})
