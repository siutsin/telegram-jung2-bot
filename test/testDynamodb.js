import test from 'ava'
import path from 'path'
import dotenv from 'dotenv'
import AWS from 'aws-sdk-mock'
import DynamoDB from '../src/dynamodb'
import stubTelegramNewMessage from './stub/telegramNewMessage'

dotenv.config({ path: path.resolve(__dirname, '.env.testing') })

const stubPutMessage = 'successfully put item into the database'
const stubQueryMessage = { Items: 'successfully query items from the database' }

test.before(t => {
  AWS.mock('DynamoDB.DocumentClient', 'put', (params, callback) => {
    callback(null, stubPutMessage)
  })
  AWS.mock('DynamoDB.DocumentClient', 'query', (params, callback) => {
    callback(null, stubQueryMessage)
  })
})

test.after.always(t => {
  AWS.restore()
})

test('saveMessage', async t => {
  const dynamodb = new DynamoDB()
  const response = await dynamodb.saveMessage(stubTelegramNewMessage.message)
  t.is(response, stubPutMessage)
})

test('getRowsByChatId', async t => {
  const dynamodb = new DynamoDB()
  const response = await dynamodb.getRowsByChatId(123)
  t.is(response, stubQueryMessage.Items)
})

test('In serverless-offline environment', async t => {
  const cache = process.env.IS_OFFLINE
  process.env.IS_OFFLINE = true
  const dynamodb = new DynamoDB()
  const response = await dynamodb.saveMessage(stubTelegramNewMessage.message)
  t.is(response, stubPutMessage)
  process.env.IS_OFFLINE = cache
})
