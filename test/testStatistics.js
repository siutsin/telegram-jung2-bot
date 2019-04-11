import test from 'ava'
import path from 'path'
import dotenv from 'dotenv'
import AWS from 'aws-sdk-mock'
import nock from 'nock'
import Statistics from '../src/statistics'
import stubTopTen from './stub/telegramMessageTopTen'
import stubAllJung from './stub/telegramMessageAllJung'
import stubAllJungDBResponse from './stub/allJungDatabaseResponse'
import stuballJungMessageResponse from './stub/allJungMessageResponse'

dotenv.config({ path: path.resolve(__dirname, '.env.testing') })

test.before(t => {
  AWS.mock('DynamoDB.DocumentClient', 'query', (params, callback) => {
    callback(null, stubAllJungDBResponse)
  })
})

test.after.always(t => {
  AWS.restore()
})

test('/topten', async t => {
  nock(`https://api.telegram.org/bot${process.env.TELEGRAM_BOT_TOKEN}`)
    .post('/sendMessage')
    .reply(200, {
      data: stuballJungMessageResponse
    })
  const statistics = new Statistics()
  const response = await statistics.topTen(stubTopTen.message)
  t.regex(response, /Top [0-9]+ 冗員s in the last 7 days/)
  t.regex(response, /1\. [a-zA-Z0-9 .]+% \(.*\)/)
  t.regex(response, /Total messages: [1-9]+[0-9]*/)
  nock.restore()
})

test('/alljung', async t => {
  nock(`https://api.telegram.org/bot${process.env.TELEGRAM_BOT_TOKEN}`)
    .post('/sendMessage')
    .reply(200, {
      data: stuballJungMessageResponse
    })
  const statistics = new Statistics()
  const response = await statistics.allJung(stubAllJung.message)
  t.regex(response, /All 冗員s in the last 7 days/)
  t.regex(response, /1\. [a-zA-Z0-9 .]+% \(.*\)/)
  t.regex(response, /Total messages: [1-9]+[0-9]*/)
  nock.restore()
})

test.serial('/topten with error', async t => {
  const statistics = new Statistics()
  try {
    await statistics.topTen(stubTopTen.message)
    t.fail('This case should throw an error')
  } catch (e) {
    t.is(e.message, 'Request failed with status code 404')
  }
})
