import test from 'ava'
import path from 'path'
import dotenv from 'dotenv'
import nock from 'nock'
import AWS from 'aws-sdk-mock'
import SQS from '../src/sqs'

import stubSQSResponse from './stub/sqsResponse'
import stubJungHelpSQSEvent from './stub/onJungHelpSQSEvent'
import stubAllJungSQSEvent from './stub/onAllJungSQSEvent'
import stubTopTenSQSEvent from './stub/onTopTenSQSEvent'
import stubTopDiverSQSEvent from './stub/onTopDiverSQSEvent'
import stubOffFromWorkSQSEvent from './stub/onOffFromWorkSQSEvent'
import stubEnableAllJungSQSEvent from './stub/onEnableAllJungSQSEvent'
import stubDisableAllJungSQSEvent from './stub/onDisableAllJungSQSEvent'
import stubAllJungMessageResponse from './stub/allJungMessageResponse'
import stubAllJungDBResponse from './stub/allJungDatabaseResponse'
import stubGetChatAdministratorsResponse from './stub/getChatAdministratorsResponse'
import stubDynamoDBQueryStatsByChatIdResponse from './stub/dynamoDBQueryStatsByChatIdResponse'

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

test('onEvent - junghelp', async t => {
  nock(`https://api.telegram.org/bot${process.env.TELEGRAM_BOT_TOKEN}`)
    .persist()
    .post('/sendMessage')
    .reply(200, stubAllJungMessageResponse)
  const sqs = new SQS()
  const response = await sqs.onEvent(stubJungHelpSQSEvent)
  t.is(response, stubDeleteMessage)
})

test('onEvent - alljung', async t => {
  nock(`https://api.telegram.org/bot${process.env.TELEGRAM_BOT_TOKEN}`)
    .persist()
    .post('/sendMessage')
    .reply(200, stubAllJungMessageResponse)
  const sqs = new SQS()
  const response = await sqs.onEvent(stubAllJungSQSEvent)
  t.is(response, stubDeleteMessage)
})

test('onEvent - topten', async t => {
  nock(`https://api.telegram.org/bot${process.env.TELEGRAM_BOT_TOKEN}`)
    .persist()
    .post('/sendMessage')
    .reply(200, stubAllJungMessageResponse)
  const sqs = new SQS()
  const response = await sqs.onEvent(stubTopTenSQSEvent)
  t.is(response, stubDeleteMessage)
})

test('onEvent - topdiver', async t => {
  nock(`https://api.telegram.org/bot${process.env.TELEGRAM_BOT_TOKEN}`)
    .persist()
    .post('/sendMessage')
    .reply(200, stubAllJungMessageResponse)
  const sqs = new SQS()
  const response = await sqs.onEvent(stubTopDiverSQSEvent)
  t.is(response, stubDeleteMessage)
})

test('onEvent - offFromWork', async t => {
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
  clone.Records[0]['messageAttributes'].allAdmin.stringValue = '1'
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
  clone.Records[0]['messageAttributes'].allAdmin.stringValue = '1'
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
