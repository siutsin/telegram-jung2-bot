import test from 'ava'
import path from 'path'
import dotenv from 'dotenv'
import AWS from 'aws-sdk-mock'
import nock from 'nock'
import Messages from '../src/messages'
import stubEvent from './stub/onMessageEvent'
import stubTopTenEvent from './stub/onTopTenMessageEvent'
import stubAllJungEvent from './stub/onAllJungMessageEvent'
import stubHelpEvent from './stub/onHelpMessageEvent'
import stubEditMessageEvent from './stub/onEditMessageEvent'
import stubAllJungMessageResponse from './stub/allJungMessageResponse'
import stubAllJungDatabaseResponse from './stub/allJungDatabaseResponse'

dotenv.config({ path: path.resolve(__dirname, '.env.testing') })

test.beforeEach(async t => {
  AWS.mock('DynamoDB.DocumentClient', 'put', (params, callback) => {
    callback(null, { Items: 'successfully query items from the database' })
  })
  AWS.mock('DynamoDB.DocumentClient', 'query', (params, callback) => {
    callback(null, stubAllJungDatabaseResponse)
  })
})

test.afterEach.always(async t => {
  AWS.restore()
  nock.cleanAll()
})

test('newMessage', async t => {
  const messages = new Messages()
  const response = await messages.newMessage(stubEvent)
  t.is(response.statusCode, 200)
})

test('newMessage - /junghelp', async t => {
  nock(`https://api.telegram.org/bot${process.env.TELEGRAM_BOT_TOKEN}`)
    .post('/sendMessage')
    .reply(200, {
      data: stubAllJungMessageResponse
    })
  const messages = new Messages()
  const response = await messages.newMessage(stubHelpEvent)
  t.is(response.statusCode, 200)
})

test('newMessage - /topten', async t => {
  nock(`https://api.telegram.org/bot${process.env.TELEGRAM_BOT_TOKEN}`)
    .post('/sendMessage')
    .reply(200, {
      data: stubAllJungMessageResponse
    })
  const messages = new Messages()
  const response = await messages.newMessage(stubTopTenEvent)
  t.is(response.statusCode, 200)
})

test('newMessage - /alljung', async t => {
  nock(`https://api.telegram.org/bot${process.env.TELEGRAM_BOT_TOKEN}`)
    .post('/sendMessage')
    .reply(200, {
      data: stubAllJungMessageResponse
    })
  const messages = new Messages()
  const response = await messages.newMessage(stubAllJungEvent)
  t.is(response.statusCode, 200)
})

test('newMessage - edit_message', async t => {
  const messages = new Messages()
  const response = await messages.newMessage(stubEditMessageEvent)
  t.is(response.statusCode, 204)
})

test('newMessage - error', async t => {
  const messages = new Messages()
  const response = await messages.newMessage({})
  t.is(response.statusCode, 500)
})
