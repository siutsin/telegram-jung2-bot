import test from 'ava'
import path from 'path'
import AWS from 'aws-sdk-mock'
import dotenv from 'dotenv'
import OffFromWork from '../src/offFromWork'

import stubChatIdScanResponse from './stub/chatIdScanResponse'
import stubSQSResponse from './stub/sqsResponse'

dotenv.config({ path: path.resolve(__dirname, '.env.testing') })

test.beforeEach(() => {
  AWS.mock('DynamoDB.DocumentClient', 'scan', (params, callback) => {
    callback(null, stubChatIdScanResponse)
  })
  AWS.mock('SQS', 'sendMessage', (params, callback) => {
    callback(null, stubSQSResponse)
  })
})

test.afterEach.always(() => {
  AWS.restore()
})

test.serial('off', async t => {
  const offFromWork = new OffFromWork()
  const response = await offFromWork.off()
  t.truthy(response)
})
