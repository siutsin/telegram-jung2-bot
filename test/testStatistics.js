import test from 'ava'
import path from 'path'
import dotenv from 'dotenv'
import AWS from 'aws-sdk-mock'
import nock from 'nock'
import Statistics from '../src/statistics'
import stubTopTen from './stub/telegramMessageTopTen'
import stubTopDiver from './stub/telegramMessageTopDiver'
import stubAllJung from './stub/telegramMessageAllJung'
import stubAllJungDBResponse from './stub/allJungDatabaseResponse'
import stubAllJungMessageResponse from './stub/allJungMessageResponse'

dotenv.config({ path: path.resolve(__dirname, '.env.testing') })

test.before(t => {
  AWS.mock('DynamoDB.DocumentClient', 'query', (params, callback) => {
    callback(null, stubAllJungDBResponse)
  })
  AWS.mock('DynamoDB.DocumentClient', 'update', (params, callback) => {
    callback(null, { Items: 'successfully update items to the database' })
  })
})

test.afterEach.always(t => {
  nock.cleanAll()
})

test.after.always(t => {
  AWS.restore()
})

test('/topten', async t => {
  nock(`https://api.telegram.org/bot${process.env.TELEGRAM_BOT_TOKEN}`)
    .post('/sendMessage')
    .reply(200, {
      data: stubAllJungMessageResponse
    })
  const statistics = new Statistics()
  const response = await statistics.topTen(stubTopTen.message.chat.id)
  t.regex(response, /Top [0-9]+ 冗員s in the last 7 days/)
  t.regex(response, /1\. [a-zA-Z0-9 .]+% \(.*\)/)
  t.regex(response, /2\. [a-zA-Z0-9 .]+% \(.*\)/)
  t.regex(response, /3\. [a-zA-Z0-9 .]+% \(.*\)/)
  t.regex(response, /4\. [a-zA-Z0-9 .]+% \(.*\)/)
  t.regex(response, /5\. [a-zA-Z0-9 .]+% \(.*\)/)
  t.regex(response, /6\. [a-zA-Z0-9 .]+% \(.*\)/)
  t.regex(response, /7\. [a-zA-Z0-9 .]+% \(.*\)/)
  t.regex(response, /8\. [a-zA-Z0-9 .]+% \(.*\)/)
  t.regex(response, /9\. [a-zA-Z0-9 .]+% \(.*\)/)
  t.regex(response, /10\. [a-zA-Z0-9 .]+% \(.*\)/)
  t.regex(response, /Total messages: [1-9]+[0-9]*/)
  const shouldNotHave11 = /11\. [a-zA-Z0-9 .]+% \(.*\)/.test(response)
  t.falsy(shouldNotHave11, 'should not have 11')
})

test('/topdiver', async t => {
  nock(`https://api.telegram.org/bot${process.env.TELEGRAM_BOT_TOKEN}`)
    .post('/sendMessage')
    .reply(200, {
      data: stubAllJungMessageResponse
    })
  const statistics = new Statistics()
  const response = await statistics.topDiver(stubTopDiver.message.chat.id)
  t.regex(response, /Top [0-9]+ 潛水員s in the last 7 days/)
  t.regex(response, /1\. [a-zA-Z0-9 .]+% \(.*\)/)
  t.regex(response, /2\. [a-zA-Z0-9 .]+% \(.*\)/)
  t.regex(response, /3\. [a-zA-Z0-9 .]+% \(.*\)/)
  t.regex(response, /4\. [a-zA-Z0-9 .]+% \(.*\)/)
  t.regex(response, /5\. [a-zA-Z0-9 .]+% \(.*\)/)
  t.regex(response, /6\. [a-zA-Z0-9 .]+% \(.*\)/)
  t.regex(response, /7\. [a-zA-Z0-9 .]+% \(.*\)/)
  t.regex(response, /8\. [a-zA-Z0-9 .]+% \(.*\)/)
  t.regex(response, /9\. [a-zA-Z0-9 .]+% \(.*\)/)
  t.regex(response, /10\. [a-zA-Z0-9 .]+% \(.*\)/)
  t.regex(response, /Total messages: [1-9]+[0-9]*/)
  const shouldNotHave11 = /11\. [a-zA-Z0-9 .]+% \(.*\)/.test(response)
  t.falsy(shouldNotHave11, 'should not have 11')
})

test('/alljung', async t => {
  nock(`https://api.telegram.org/bot${process.env.TELEGRAM_BOT_TOKEN}`)
    .post('/sendMessage')
    .reply(200, {
      data: stubAllJungMessageResponse
    })
  const statistics = new Statistics()
  const response = await statistics.allJung(stubAllJung.message.chat.id)
  t.regex(response, /All 冗員s in the last 7 days/)
  t.regex(response, /1\. [a-zA-Z0-9 .]+% \(.*\)/)
  t.regex(response, /2\. [a-zA-Z0-9 .]+% \(.*\)/)
  t.regex(response, /3\. [a-zA-Z0-9 .]+% \(.*\)/)
  t.regex(response, /4\. [a-zA-Z0-9 .]+% \(.*\)/)
  t.regex(response, /5\. [a-zA-Z0-9 .]+% \(.*\)/)
  t.regex(response, /6\. [a-zA-Z0-9 .]+% \(.*\)/)
  t.regex(response, /7\. [a-zA-Z0-9 .]+% \(.*\)/)
  t.regex(response, /8\. [a-zA-Z0-9 .]+% \(.*\)/)
  t.regex(response, /9\. [a-zA-Z0-9 .]+% \(.*\)/)
  t.regex(response, /10\. [a-zA-Z0-9 .]+% \(.*\)/)
  t.regex(response, /11\. [a-zA-Z0-9 .]+% \(.*\)/)
  t.regex(response, /Total messages: [1-9]+[0-9]*/)
})

test('/topten with 4xx error', async t => {
  nock(`https://api.telegram.org/bot${process.env.TELEGRAM_BOT_TOKEN}`)
    .post('/sendMessage')
    .reply(497, 'Request failed with status code 497')
  const statistics = new Statistics()
  const result = await statistics.topTen(stubTopTen.message.chat.id)
  t.truthy(result)
})

test('/topten with 9xx error', async t => {
  nock(`https://api.telegram.org/bot${process.env.TELEGRAM_BOT_TOKEN}`)
    .post('/sendMessage')
    .reply(996, 'Request failed with status code 996')
  const statistics = new Statistics()
  try {
    await statistics.topTen(stubTopTen.message.chat.id)
    t.fail('This case should throw an error')
  } catch (e) {
    t.is(e.message, 'Request failed with status code 996')
  }
})
