import test from 'ava'
import path from 'path'
import dotenv from 'dotenv'
import AWS from 'aws-sdk-mock'
import DynamoDB from '../src/dynamodb'
import stubTelegramNewMessage from './stub/telegramNewMessage'

dotenv.config({ path: path.resolve(__dirname, '.env.testing') })

test.afterEach(t => {
  AWS.restore()
})

test('saveMessage', async t => {
  const stubMessage = 'successfully put item into the database'
  AWS.mock('DynamoDB.DocumentClient', 'put', (params, callback) => {
    callback(null, stubMessage)
  })
  const dynamodb = new DynamoDB()
  const response = await dynamodb.saveMessage(stubTelegramNewMessage.message)
  t.is(response, stubMessage)
})

test('getRowsByChatId', async t => {
  const stubMessage = { Items: 'successfully query items from the database' }
  AWS.mock('DynamoDB.DocumentClient', 'query', (params, callback) => {
    callback(null, stubMessage)
  })
  const dynamodb = new DynamoDB()
  const response = await dynamodb.getRowsByChatId(123)
  t.is(response, stubMessage.Items)
})

test('In serverless-offline environment', async t => {
  const cache = process.env.IS_OFFLINE
  process.env.IS_OFFLINE = true
  const stubMessage = 'successfully put item into the database'
  AWS.mock('DynamoDB.DocumentClient', 'put', (params, callback) => {
    callback(null, stubMessage)
  })
  const dynamodb = new DynamoDB()
  const response = await dynamodb.saveMessage(stubTelegramNewMessage.message)
  t.is(response, stubMessage)
  process.env.IS_OFFLINE = cache
})
