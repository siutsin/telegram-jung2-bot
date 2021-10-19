const test = require('ava')
const AWS = require('aws-sdk-mock')
const path = require('path')
const dotenv = require('dotenv')
const nock = require('nock')

const SQS = require('../src/sqs')

const stubSQSResponse = require('./stub/sqsResponse')
const stubJungHelpSQSEvent = require('./stub/onJungHelpSQSEvent')
const stubAllJungSQSEvent = require('./stub/onAllJungSQSEvent')
const stubTopTenSQSEvent = require('./stub/onTopTenSQSEvent')
const stubTopDiverSQSEvent = require('./stub/onTopDiverSQSEvent')
const stubOffFromWorkSQSEvent = require('./stub/onOffFromWorkSQSEvent')
const stubEnableAllJungSQSEvent = require('./stub/onEnableAllJungSQSEvent')
const stubDisableAllJungSQSEvent = require('./stub/onDisableAllJungSQSEvent')
const stubAllJungMessageResponse = require('./stub/allJungMessageResponse')
const stubAllJungDBResponse = require('./stub/allJungDatabaseResponse')
const stubGetChatAdministratorsResponse = require('./stub/getChatAdministratorsResponse')
const stubDynamoDBQueryStatsByChatIdResponse = require('./stub/dynamoDBQueryStatsByChatIdResponse')

dotenv.config({ path: path.resolve(__dirname, '.env.testing') })

const stubDeleteMessage = { Dummy: 'deleteMessage' }

test.beforeEach(() => {
  AWS.mock('SQS', 'sendMessage', (params, callback) => {
    callback(null, stubSQSResponse)
  })
  AWS.mock('SQS', 'deleteMessage', (params, callback) => {
    callback(null, stubDeleteMessage)
  })
  AWS.mock('DynamoDB.DocumentClient', 'query', (params, callback) => {
    if (params.TableName === process.env.MESSAGE_TABLE) {
      callback(null, stubAllJungDBResponse)
    } else {
      callback(null, stubDynamoDBQueryStatsByChatIdResponse)
    }
  })
  AWS.mock('DynamoDB.DocumentClient', 'update', (params, callback) => {
    callback(null, { Items: 'successfully update items to the database' })
  })
})

test.afterEach.always(() => {
  nock.cleanAll()
  AWS.restore()
})

test.serial('onEvent - junghelp', async t => {
  nock(`https://api.telegram.org/bot${process.env.TELEGRAM_BOT_TOKEN}`)
    .persist()
    .post('/sendMessage')
    .reply(200, stubAllJungMessageResponse)
  const sqs = new SQS()
  const response = await sqs.onEvent(stubJungHelpSQSEvent)
  t.is(response, stubDeleteMessage)
})

test.serial('onEvent - alljung', async t => {
  nock(`https://api.telegram.org/bot${process.env.TELEGRAM_BOT_TOKEN}`)
    .persist()
    .post('/sendMessage')
    .reply(200, stubAllJungMessageResponse)
  const sqs = new SQS()
  const response = await sqs.onEvent(stubAllJungSQSEvent)
  t.is(response, stubDeleteMessage)
})

test.serial('onEvent - topten', async t => {
  nock(`https://api.telegram.org/bot${process.env.TELEGRAM_BOT_TOKEN}`)
    .persist()
    .post('/sendMessage')
    .reply(200, stubAllJungMessageResponse)
  const sqs = new SQS()
  const response = await sqs.onEvent(stubTopTenSQSEvent)
  t.is(response, stubDeleteMessage)
})

test.serial('onEvent - topdiver', async t => {
  nock(`https://api.telegram.org/bot${process.env.TELEGRAM_BOT_TOKEN}`)
    .persist()
    .post('/sendMessage')
    .reply(200, stubAllJungMessageResponse)
  const sqs = new SQS()
  const response = await sqs.onEvent(stubTopDiverSQSEvent)
  t.is(response, stubDeleteMessage)
})

test.serial('onEvent - offFromWork', async t => {
  nock(`https://api.telegram.org/bot${process.env.TELEGRAM_BOT_TOKEN}`)
    .persist()
    .post('/sendMessage')
    .reply(200, stubAllJungMessageResponse)
  const sqs = new SQS()
  const response = await sqs.onEvent(stubOffFromWorkSQSEvent)
  t.is(response, stubDeleteMessage)
})

test.serial('onEvent - enableAllJung', async t => {
  nock(`https://api.telegram.org/bot${process.env.TELEGRAM_BOT_TOKEN}`)
    .persist()
    .post('/sendMessage')
    .reply(200, stubAllJungMessageResponse)
  nock(`https://api.telegram.org/bot${process.env.TELEGRAM_BOT_TOKEN}`)
    .persist()
    .get('/getChatAdministrators')
    .query({ chat_id: -123 })
    .reply(200, stubGetChatAdministratorsResponse)
  const sqs = new SQS()
  const response = await sqs.onEvent(stubEnableAllJungSQSEvent)
  t.is(response, stubDeleteMessage)
})

test.serial('onEvent - enableAllJung - not admin', async t => {
  nock(`https://api.telegram.org/bot${process.env.TELEGRAM_BOT_TOKEN}`)
    .persist()
    .post('/sendMessage')
    .reply(200, stubAllJungMessageResponse)
  const clone = JSON.parse(JSON.stringify(stubGetChatAdministratorsResponse))
  clone.result[0].user.id = 999
  nock(`https://api.telegram.org/bot${process.env.TELEGRAM_BOT_TOKEN}`)
    .persist()
    .get('/getChatAdministrators')
    .query({ chat_id: -123 })
    .reply(200, clone)
  const sqs = new SQS()
  const response = await sqs.onEvent(stubEnableAllJungSQSEvent)
  t.is(response, stubDeleteMessage)
})

test.serial('onEvent - enableAllJung - all admin', async t => {
  nock(`https://api.telegram.org/bot${process.env.TELEGRAM_BOT_TOKEN}`)
    .persist()
    .post('/sendMessage')
    .reply(200, stubAllJungMessageResponse)
  nock(`https://api.telegram.org/bot${process.env.TELEGRAM_BOT_TOKEN}`)
    .persist()
    .get('/getChatAdministrators')
    .query({ chat_id: -123 })
    .reply(200, stubGetChatAdministratorsResponse)
  const clone = JSON.parse(JSON.stringify(stubEnableAllJungSQSEvent))
  clone.Records[0].messageAttributes.allAdmin.stringValue = '1'
  const sqs = new SQS()
  const response = await sqs.onEvent(clone)
  t.is(response, stubDeleteMessage)
})

test.serial('onEvent - disableAllJung', async t => {
  nock(`https://api.telegram.org/bot${process.env.TELEGRAM_BOT_TOKEN}`)
    .persist()
    .post('/sendMessage')
    .reply(200, stubAllJungMessageResponse)
  nock(`https://api.telegram.org/bot${process.env.TELEGRAM_BOT_TOKEN}`)
    .persist()
    .get('/getChatAdministrators')
    .query({ chat_id: -123 })
    .reply(200, stubGetChatAdministratorsResponse)
  const sqs = new SQS()
  const response = await sqs.onEvent(stubDisableAllJungSQSEvent)
  t.is(response, stubDeleteMessage)
})

test.serial('onEvent - disableAllJung - not admin', async t => {
  nock(`https://api.telegram.org/bot${process.env.TELEGRAM_BOT_TOKEN}`)
    .persist()
    .post('/sendMessage')
    .reply(200, stubAllJungMessageResponse)
  const clone = JSON.parse(JSON.stringify(stubGetChatAdministratorsResponse))
  clone.result[0].user.id = 999
  nock(`https://api.telegram.org/bot${process.env.TELEGRAM_BOT_TOKEN}`)
    .persist()
    .get('/getChatAdministrators')
    .query({ chat_id: -123 })
    .reply(200, clone)
  const sqs = new SQS()
  const response = await sqs.onEvent(stubDisableAllJungSQSEvent)
  t.is(response, stubDeleteMessage)
})

test.serial('onEvent - disableAllJung - all admin', async t => {
  nock(`https://api.telegram.org/bot${process.env.TELEGRAM_BOT_TOKEN}`)
    .persist()
    .post('/sendMessage')
    .reply(200, stubAllJungMessageResponse)
  nock(`https://api.telegram.org/bot${process.env.TELEGRAM_BOT_TOKEN}`)
    .persist()
    .get('/getChatAdministrators')
    .query({ chat_id: -123 })
    .reply(200, stubGetChatAdministratorsResponse)
  const clone = JSON.parse(JSON.stringify(stubDisableAllJungSQSEvent))
  clone.Records[0].messageAttributes.allAdmin.stringValue = '1'
  const sqs = new SQS()
  const response = await sqs.onEvent(clone)
  t.is(response, stubDeleteMessage)
})

test.serial('onEvent - junghelp with error', async t => {
  nock(`https://api.telegram.org/bot${process.env.TELEGRAM_BOT_TOKEN}`)
    .persist()
    .post('/sendMessage')
    .reply(987, 'Request failed with status code 987')
  const sqs = new SQS()
  const response = await sqs.onEvent(stubJungHelpSQSEvent)
  t.is(response, 'Request failed with status code 987')
})

// In ECS SQS polling, the key is `StringValue` instead of `stringValue`.
test.serial('onEvent - alljung - ecs mode', async t => {
  nock(`https://api.telegram.org/bot${process.env.TELEGRAM_BOT_TOKEN}`)
    .persist()
    .post('/sendMessage')
    .reply(200, stubAllJungMessageResponse)
  const sqs = new SQS()

  const copy = JSON.parse(JSON.stringify(stubAllJungSQSEvent))
  copy.Records[0].messageAttributes.chatId.StringValue = copy.Records[0].messageAttributes.chatId.stringValue
  copy.Records[0].messageAttributes.action.StringValue = copy.Records[0].messageAttributes.action.stringValue
  delete copy.Records[0].messageAttributes.chatId.stringValue
  delete copy.Records[0].messageAttributes.action.stringValue

  const response = await sqs.onEvent(copy)
  t.is(response, stubDeleteMessage)
})
