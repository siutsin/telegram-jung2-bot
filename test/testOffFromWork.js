const test = require('ava')
const AWS = require('aws-sdk-mock')
const path = require('path')
const dotenv = require('dotenv')
const OffFromWork = require('../src/offFromWork')

const stubChatIdScanResponse = require('./stub/chatIdScanResponse')
const stubSQSResponse = require('./stub/sqsResponse')

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
  const timeString = '2022-03-04T10:00:00.000Z'
  const response = await offFromWork.off(timeString)
  t.truthy(response)
})
