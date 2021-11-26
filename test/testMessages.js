const test = require('ava')
const AWS = require('aws-sdk-mock')
const path = require('path')
const dotenv = require('dotenv')
const nock = require('nock')
const Messages = require('../src/messages')
const stubEvent = require('./stub/onMessageEvent')
const stubTopTenEvent = require('./stub/onTopTenMessageEvent')
const stubTopDiverEvent = require('./stub/onTopDiverMessageEvent')
const stubAllJungEvent = require('./stub/onAllJungMessageEvent')
const stubHelpEvent = require('./stub/onHelpMessageEvent')
const stubEditMessageEvent = require('./stub/onEditMessageEvent')
const stubEnableAllJungEvent = require('./stub/onEnableAllJungMessageEvent')
const stubDisableAllJungEvent = require('./stub/onDisableAllJungMessageEvent')
const stubAllJungMessageResponse = require('./stub/allJungMessageResponse')
const stubSQSResponse = require('./stub/sqsResponse')

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

test('newMessage = require( Telegram IP range', async t => {
  const messages = new Messages()
  const clone = JSON.parse(JSON.stringify(stubEvent))
  clone.headers['X-Forwarded-For'] = '149.154.160.1, 1.2.3.4'
  let response = await messages.newMessage(clone)
  t.is(response.statusCode, 200)
  clone.headers = { 'x-forwarded-for': '91.108.4.1, 1.2.3.4' }
  response = await messages.newMessage(clone)
  t.is(response.statusCode, 200)
})

test('newMessage not = require( Telegram IP range', async t => {
  const messages = new Messages()
  const clone = JSON.parse(JSON.stringify(stubEvent))
  clone.headers['X-Forwarded-For'] = '1.2.3.4, 1.2.3.4'
  const response = await messages.newMessage(clone)
  t.is(response.statusCode, 403)
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
