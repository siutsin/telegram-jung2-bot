import test from 'ava'
import path from 'path'
import AWS from 'aws-sdk-mock'
import nock from 'nock'
import OffFromWork from '../src/offFromWork'
import dotenv from 'dotenv'
import stubAllJungDatabaseResponseReverseOrder from './stub/allJungDatabaseResponseReverseOrder'
import stubAllJungMessageResponse from './stub/allJungMessageResponse'

dotenv.config({ path: path.resolve(__dirname, '.env.testing') })

const stubQueryMessage = { Items: 'successfully query items from the database' }

test.before(t => {
  AWS.mock('DynamoDB.DocumentClient', 'scan', (params, callback) => {
    callback(null, stubAllJungDatabaseResponseReverseOrder)
  })
  AWS.mock('DynamoDB.DocumentClient', 'query', (params, callback) => {
    callback(null, stubQueryMessage)
  })
})

test.afterEach.always(async t => {
  nock.cleanAll()
})

test.after.always(t => {
  AWS.restore()
})

test('off', async t => {
  nock(`https://api.telegram.org/bot${process.env.TELEGRAM_BOT_TOKEN}`)
    .persist()
    .post('/sendMessage')
    .reply(200, {
      data: stubAllJungMessageResponse
    })
  const offFromWork = new OffFromWork()
  const response = await offFromWork.off()
  t.truthy(response)
})

test.serial('off - should catch 4xx and 5xx error', async t => {
  nock(`https://api.telegram.org/bot${process.env.TELEGRAM_BOT_TOKEN}`)
    .persist()
    .post('/sendMessage')
    .reply(498, 'Request failed with status code 498')
  const offFromWork = new OffFromWork()
  const response = await offFromWork.off()
  t.truthy(response)
})

test.serial('off - with 999 error', async t => {
  nock(`https://api.telegram.org/bot${process.env.TELEGRAM_BOT_TOKEN}`)
    .persist()
    .post('/sendMessage')
    .reply(999, 'Request failed with status code 999')
  try {
    const offFromWork = new OffFromWork()
    const response = await offFromWork.off()
    t.falsy(response)
  } catch (e) {
    t.is(e.message, 'Request failed with status code 999')
  }
})
