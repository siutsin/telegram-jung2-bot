import test from 'ava'
import path from 'path'
import dotenv from 'dotenv'
import nock from 'nock'
import AWS from 'aws-sdk-mock'
import SQS from '../src/sqs'

import stubSQSResponse from './stub/sqsResponse'
import stubTopTenEvent from './stub/onTopTenMessageEvent'
import stubAllJungSQSEvent from './stub/onAllJungSQSEvent'
import stubTopTenSQSEvent from './stub/onTopTenSQSEvent'
import stubAllJungMessageResponse from './stub/allJungMessageResponse'
import stubAllJungDBResponse from './stub/allJungDatabaseResponse'

dotenv.config({ path: path.resolve(__dirname, '.env.testing') })

const stubDeleteMessage = { Dummy: 'deleteMessage' }

test.before(t => {
  AWS.mock('SQS', 'sendMessage', (params, callback) => {
    callback(null, stubSQSResponse)
  })
  AWS.mock('SQS', 'deleteMessage', (params, callback) => {
    callback(null, stubDeleteMessage)
  })
  AWS.mock('DynamoDB.DocumentClient', 'query', (params, callback) => {
    callback(null, stubAllJungDBResponse)
  })
})

test.afterEach.always(t => {
  nock.cleanAll()
})

test.after.always(t => {
  AWS.restore()
})

test('sendTopTenMessage', async t => {
  const event = JSON.parse(stubTopTenEvent.body)
  const message = event.message
  const sqs = new SQS()
  const response = await sqs.sendTopTenMessage(message)
  t.is(response, stubSQSResponse)
})

test('sendAllJungMessage', async t => {
  const event = JSON.parse(stubTopTenEvent.body)
  const message = event.message
  const sqs = new SQS()
  const response = await sqs.sendAllJungMessage(message)
  t.is(response, stubSQSResponse)
})

test('onEvent - alljung', async t => {
  nock(`https://api.telegram.org/bot${process.env.TELEGRAM_BOT_TOKEN}`)
    .post('/sendMessage')
    .reply(200, {
      data: stubAllJungMessageResponse
    })
  const sqs = new SQS()
  const response = await sqs.onEvent(stubAllJungSQSEvent)
  t.is(response, stubDeleteMessage)
})

test('onEvent - topten', async t => {
  nock(`https://api.telegram.org/bot${process.env.TELEGRAM_BOT_TOKEN}`)
    .post('/sendMessage')
    .reply(200, {
      data: stubAllJungMessageResponse
    })
  const sqs = new SQS()
  const response = await sqs.onEvent(stubTopTenSQSEvent)
  t.is(response, stubDeleteMessage)
})
