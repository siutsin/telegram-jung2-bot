import test from 'ava'
import path from 'path'
import dotenv from 'dotenv'
import AWS from 'aws-sdk-mock'
import nock from 'nock'
import Messages from '../src/messages'
import stubEvent from './stub/onMessageEvent'
import stubTopTenEvent from './stub/onTopTenMessageEvent'
import stubTopDiverEvent from './stub/onTopDiverMessageEvent'
import stubAllJungEvent from './stub/onAllJungMessageEvent'
import stubHelpEvent from './stub/onHelpMessageEvent'
import stubEditMessageEvent from './stub/onEditMessageEvent'
import stubEnableAllJungEvent from './stub/onEnableAllJungMessageEvent'
import stubDisableAllJungEvent from './stub/onDisableAllJungMessageEvent'
import stubAllJungMessageResponse from './stub/allJungMessageResponse'
import stubSQSResponse from './stub/sqsResponse'

dotenv.config({ path: path.resolve(__dirname, '.env.testing') })

test.beforeEach(async () => {
  AWS.mock('DynamoDB.DocumentClient', 'update', (params, callback) => {
    callback(null, { Items: 'successfully update items to the database' })
  })
  AWS.mock('SQS', 'sendMessage', (params, callback) => {
    callback(null, stubSQSResponse)
  })
})

test.afterEach.always(async () => {
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
    .persist()
    .post('/sendMessage')
    .reply(200, stubAllJungMessageResponse)
  const messages = new Messages()
  const response = await messages.newMessage(stubHelpEvent)
  t.is(response.statusCode, 200)
})

test('newMessage - /topten', async t => {
  nock(`https://api.telegram.org/bot${process.env.TELEGRAM_BOT_TOKEN}`)
    .persist()
    .post('/sendMessage')
    .reply(200, stubAllJungMessageResponse)
  const messages = new Messages()
  const response = await messages.newMessage(stubTopTenEvent)
  t.is(response.statusCode, 200)
})

test('newMessage - /topdiver', async t => {
  nock(`https://api.telegram.org/bot${process.env.TELEGRAM_BOT_TOKEN}`)
    .persist()
    .post('/sendMessage')
    .reply(200, stubAllJungMessageResponse)
  const messages = new Messages()
  const response = await messages.newMessage(stubTopDiverEvent)
  t.is(response.statusCode, 200)
})

test('newMessage - /alljung', async t => {
  nock(`https://api.telegram.org/bot${process.env.TELEGRAM_BOT_TOKEN}`)
    .persist()
    .post('/sendMessage')
    .reply(200, stubAllJungMessageResponse)
  const messages = new Messages()
  const response = await messages.newMessage(stubAllJungEvent)
  t.is(response.statusCode, 200)
})

test('newMessage - /enableAllJung - not all admin', async t => {
  nock(`https://api.telegram.org/bot${process.env.TELEGRAM_BOT_TOKEN}`)
    .persist()
    .post('/sendMessage')
    .reply(200, stubAllJungMessageResponse)
  const clone = JSON.parse(JSON.stringify(stubEnableAllJungEvent))
  const body = JSON.parse(clone.body)
  delete body.message.chat.all_members_are_administrators
  clone.body = JSON.stringify(body)
  const messages = new Messages()
  const response = await messages.newMessage(clone)
  t.is(response.statusCode, 200)
})

test('newMessage - /enableAllJung - all admin', async t => {
  nock(`https://api.telegram.org/bot${process.env.TELEGRAM_BOT_TOKEN}`)
    .persist()
    .post('/sendMessage')
    .reply(200, stubAllJungMessageResponse)
  const messages = new Messages()
  const response = await messages.newMessage(stubEnableAllJungEvent)
  t.is(response.statusCode, 200)
})

test('newMessage - /disableAllJung - not all admin', async t => {
  nock(`https://api.telegram.org/bot${process.env.TELEGRAM_BOT_TOKEN}`)
    .persist()
    .post('/sendMessage')
    .reply(200, stubAllJungMessageResponse)
  const clone = JSON.parse(JSON.stringify(stubDisableAllJungEvent))
  const body = JSON.parse(clone.body)
  delete body.message.chat.all_members_are_administrators
  clone.body = JSON.stringify(body)
  const messages = new Messages()
  const response = await messages.newMessage(clone)
  t.is(response.statusCode, 200)
})

test('newMessage - /disableAllJung - all admin', async t => {
  nock(`https://api.telegram.org/bot${process.env.TELEGRAM_BOT_TOKEN}`)
    .persist()
    .post('/sendMessage')
    .reply(200, stubAllJungMessageResponse)
  const messages = new Messages()
  const response = await messages.newMessage(stubDisableAllJungEvent)
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
