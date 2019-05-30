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
import stubAllJungMessageResponse from './stub/allJungMessageResponse'
import stubAllJungDBResponse from './stub/allJungDatabaseResponse'

dotenv.config({ path: path.resolve(__dirname, '.env.testing') })

const stubDeleteMessage = { Dummy: 'deleteMessage' }

test.beforeEach(t => {
  AWS.mock('SQS', 'sendMessage', (params, callback) => {
    callback(null, stubSQSResponse)
  })
  AWS.mock('SQS', 'deleteMessage', (params, callback) => {
    callback(null, stubDeleteMessage)
  })
  AWS.mock('DynamoDB.DocumentClient', 'query', (params, callback) => {
    callback(null, stubAllJungDBResponse)
  })
  AWS.mock('DynamoDB.DocumentClient', 'update', (params, callback) => {
    callback(null, { Items: 'successfully update items to the database' })
  })
})

test.afterEach.always(t => {
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

test.serial('onEvent - junghelp with error', async t => {
  nock(`https://api.telegram.org/bot${process.env.TELEGRAM_BOT_TOKEN}`)
    .persist()
    .post('/sendMessage')
    .reply(400)
  const sqs = new SQS()
  const response = await sqs.onEvent(stubJungHelpSQSEvent)
  t.is(response, stubDeleteMessage)
})
