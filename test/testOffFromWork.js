import test from 'ava'
import path from 'path'
import AWS from 'aws-sdk-mock'
import nock from 'nock'
import OffFromWork from '../src/offFromWork'
import dotenv from 'dotenv'
import stubAllJungDBResponse from './stub/allJungDatabaseResponse'
import stubAllJungMessageResponse from './stub/allJungMessageResponse'

dotenv.config({ path: path.resolve(__dirname, '.env.testing') })

test.before(t => {
  AWS.mock('DynamoDB.DocumentClient', 'scan', (params, callback) => {
    callback(null, stubAllJungDBResponse)
  })
})

test.after.always(t => {
  AWS.restore()
})

test.serial('off', async t => {
  nock(`https://api.telegram.org/bot${process.env.TELEGRAM_BOT_TOKEN}`)
    .persist()
    .post('/sendMessage')
    .reply(200, {
      data: stubAllJungMessageResponse
    })
  const offFromWork = new OffFromWork()
  const response = await offFromWork.off()
  t.truthy(response)
  nock.restore()
})

test.serial('off - with error', async t => {
  nock(`https://api.telegram.org/bot${process.env.TELEGRAM_BOT_TOKEN}`)
    .persist()
    .post('/sendMessage')
    .reply(404, 'Request failed with status code 404')
  try {
    const offFromWork = new OffFromWork()
    const response = await offFromWork.off()
    t.falsy(response)
  } catch (e) {
    t.is(e.message, 'Request failed with status code 404')
  }
  nock.restore()
})
