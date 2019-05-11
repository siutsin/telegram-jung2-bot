import test from 'ava'
import path from 'path'
import AWS from 'aws-sdk-mock'
import dotenv from 'dotenv'
import OffFromWork from '../src/offFromWork'

import stubAllJungDatabaseResponseReverseOrder from './stub/allJungDatabaseResponseReverseOrder'
import stubSQSResponse from './stub/sqsResponse'

dotenv.config({ path: path.resolve(__dirname, '.env.testing') })

test.before(t => {
  AWS.mock('DynamoDB.DocumentClient', 'scan', (params, callback) => {
    callback(null, stubAllJungDatabaseResponseReverseOrder)
  })
  AWS.mock('SQS', 'sendMessage', (params, callback) => {
    callback(null, stubSQSResponse)
  })
})

test.after.always(t => {
  AWS.restore()
})

test('off', async t => {
  const offFromWork = new OffFromWork()
  const response = await offFromWork.off()
  t.truthy(response)
})
