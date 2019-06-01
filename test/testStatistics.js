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
import stubDynamoDBQueryStatsByChatIdResponse from './stub/dynamoDBQueryStatsByChatIdResponse'
import stubAllJungMessageResponse from './stub/allJungMessageResponse'

dotenv.config({ path: path.resolve(__dirname, '.env.testing') })

test.beforeEach(() => {
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

test('/topten', async t => {
  nock(`https://api.telegram.org/bot${process.env.TELEGRAM_BOT_TOKEN}`)
    .persist()
    .post('/sendMessage')
    .reply(200, {
      data: stubAllJungMessageResponse
    })
  const statistics = new Statistics()
  const response = await statistics.topTen({ chatId: stubTopTen.message.chat.id })
  t.regex(response, /Top [0-9]+ 冗員s in the last 7 days \(last 上水 time\):/)
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
  t.regex(response, /Last Update/)
  t.falsy(/11\. [a-zA-Z0-9 .]+% \(.*\)/.test(response), 'should not have 11')
})

test('/topdiver', async t => {
  nock(`https://api.telegram.org/bot${process.env.TELEGRAM_BOT_TOKEN}`)
    .persist()
    .post('/sendMessage')
    .reply(200, {
      data: stubAllJungMessageResponse
    })
  const statistics = new Statistics()
  const response = await statistics.topDiver({ chatId: stubTopDiver.message.chat.id })
  t.regex(response, /Top [0-9]+ 潛水員s in the last 7 days/)
  t.regex(response, /By 冗power:/)
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
  t.regex(response, /By last 上水:/)
  t.regex(response, /1\. [a-zA-Z0-9 .]+ - .*/)
  t.regex(response, /2\. [a-zA-Z0-9 .]+ - .*/)
  t.regex(response, /3\. [a-zA-Z0-9 .]+ - .*/)
  t.regex(response, /4\. [a-zA-Z0-9 .]+ - .*/)
  t.regex(response, /5\. [a-zA-Z0-9 .]+ - .*/)
  t.regex(response, /6\. [a-zA-Z0-9 .]+ - .*/)
  t.regex(response, /7\. [a-zA-Z0-9 .]+ - .*/)
  t.regex(response, /8\. [a-zA-Z0-9 .]+ - .*/)
  t.regex(response, /9\. [a-zA-Z0-9 .]+ - .*/)
  t.regex(response, /10\. [a-zA-Z0-9 .]+ - .*/)
  t.regex(response, /Total messages: [1-9]+[0-9]*/)
  t.regex(response, /深潛會搵唔到/)
  t.regex(response, /Last Update/)
  t.falsy(/11\. [a-zA-Z0-9 .]+% \(.*\)/.test(response), 'should not have 11')
  t.falsy(/11\. [a-zA-Z0-9 .]+ - .*/.test(response), 'should not have 11')
})

test.serial('/topdiver - less than 10 users in a group', async t => {
  AWS.remock('DynamoDB.DocumentClient', 'query', (params, callback) => {
    callback(null, {
      'Items': [
        {
          'chatTitle': 'chatTitle 2',
          'firstName': 'firstName345',
          'lastName': 'lastName345',
          'dateCreated': '2019-03-16T02:26:19+08:00',
          'chatId': 2,
          'id': '41bd62f3-3cea-48e7-9762-f3b78e1bcd88',
          'userId': 92,
          'username': 'username345'
        }
      ],
      'Count': 1,
      'ScannedCount': 1
    })
  })
  nock(`https://api.telegram.org/bot${process.env.TELEGRAM_BOT_TOKEN}`)
    .persist()
    .post('/sendMessage')
    .reply(200, {
      data: stubAllJungMessageResponse
    })
  const statistics = new Statistics()
  const response = await statistics.topDiver({ chatId: stubTopDiver.message.chat.id })
  t.regex(response, /Top [0-9]+ 潛水員s in the last 7 days/)
  t.regex(response, /By 冗power:/)
  t.regex(response, /1\. [a-zA-Z0-9 .]+% \(.*\)/)
  t.regex(response, /By last 上水:/)
  t.regex(response, /1\. [a-zA-Z0-9 .]+ - .*/)
  t.regex(response, /Total messages: [1-9]+[0-9]*/)
  t.regex(response, /深潛會搵唔到/)
  t.regex(response, /Last Update/)
  t.falsy(/2\. [a-zA-Z0-9 .]+% \(.*\)/.test(response), 'should not have 2')
  t.falsy(/2\. [a-zA-Z0-9 .]+ - .*/.test(response), 'should not have 2')
})

test('/alljung', async t => {
  nock(`https://api.telegram.org/bot${process.env.TELEGRAM_BOT_TOKEN}`)
    .persist()
    .post('/sendMessage')
    .reply(200, {
      data: stubAllJungMessageResponse
    })
  const statistics = new Statistics()
  const response = await statistics.allJung({ chatId: stubAllJung.message.chat.id })
  t.regex(response, /All 冗員s in the last 7 days \(last 上水 time\):/)
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
  t.regex(response, /Last Update/)
})

test.serial('/alljung - not enabled', async t => {
  const clone = JSON.parse(JSON.stringify(stubDynamoDBQueryStatsByChatIdResponse))
  clone.Items[0].enableAllJung = false
  AWS.remock('DynamoDB.DocumentClient', 'query', (params, callback) => {
    if (params.TableName === process.env.MESSAGE_TABLE) {
      callback(null, stubAllJungDBResponse)
    } else {
      callback(null, clone)
    }
  })
  nock(`https://api.telegram.org/bot${process.env.TELEGRAM_BOT_TOKEN}`)
    .persist()
    .post('/sendMessage')
    .reply(200, {
      data: stubAllJungMessageResponse
    })
  const statistics = new Statistics()
  const response = await statistics.allJung({ chatId: stubAllJung.message.chat.id })
  t.falsy(response)
})

test.serial('/alljung - not set', async t => {
  const clone = JSON.parse(JSON.stringify(stubDynamoDBQueryStatsByChatIdResponse))
  delete clone.Items[0].enableAllJung
  AWS.remock('DynamoDB.DocumentClient', 'query', (params, callback) => {
    if (params.TableName === process.env.MESSAGE_TABLE) {
      callback(null, stubAllJungDBResponse)
    } else {
      callback(null, clone)
    }
  })
  nock(`https://api.telegram.org/bot${process.env.TELEGRAM_BOT_TOKEN}`)
    .persist()
    .post('/sendMessage')
    .reply(200, {
      data: stubAllJungMessageResponse
    })
  const statistics = new Statistics()
  const response = await statistics.allJung({ chatId: stubAllJung.message.chat.id })
  t.falsy(response)
})

test.serial('/topten with 4xx error', async t => {
  nock(`https://api.telegram.org/bot${process.env.TELEGRAM_BOT_TOKEN}`)
    .persist()
    .post('/sendMessage')
    .reply(497, 'Request failed with status code 497')
  const statistics = new Statistics()
  const result = await statistics.topTen({ chatId: stubTopTen.message.chat.id })
  t.truthy(result)
})

test.serial('/topten with 9xx error', async t => {
  nock(`https://api.telegram.org/bot${process.env.TELEGRAM_BOT_TOKEN}`)
    .persist()
    .post('/sendMessage')
    .reply(996, 'Request failed with status code 996')
  const statistics = new Statistics()
  try {
    await statistics.topTen({ chatId: stubTopTen.message.chat.id })
    t.fail('This case should throw an error')
  } catch (e) {
    t.is(e.message, 'Request failed with status code 996')
  }
})
