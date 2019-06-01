import test from 'ava'
import path from 'path'
import dotenv from 'dotenv'
import AWS from 'aws-sdk-mock'
import DynamoDB from '../src/dynamodb'

dotenv.config({ path: path.resolve(__dirname, '.env.testing') })

test.beforeEach(() => {
  AWS.mock('DynamoDB', 'describeTable', (params, callback) => {
    callback(null, {
      Table: {
        ProvisionedThroughput: {
          WriteCapacityUnits: 1,
          ReadCapacityUnits: 1
        }
      }
    })
  })
})

test.afterEach.always(() => {
  AWS.restore()
})

test('scale up', async t => {
  AWS.mock('DynamoDB', 'updateTable', (params, callback) => {
    callback(null, {
      TableDescription: {
        TableStatus: 'UPDATING'
      }
    })
  })
  const dynamoDB = new DynamoDB()
  const response = await dynamoDB.scaleUp()
  t.is(response.TableDescription.TableStatus, 'UPDATING')
})

test.serial('scale up - Subscriber limit exceeded', async t => {
  AWS.mock('DynamoDB', 'updateTable', (params, callback) => {
    callback(new Error('Subscriber limit exceeded: Provisioned throughput decreases are limited within a given UTC day.'))
  })
  const dynamoDB = new DynamoDB()
  const response = await dynamoDB.scaleUp()
  t.truthy(response.includes('Subscriber limit exceeded'))
})

test.serial('scale up - The provisioned throughput for the table will not change', async t => {
  AWS.mock('DynamoDB', 'updateTable', (params, callback) => {
    callback(new Error('The provisioned throughput for the table will not change blah blah blah'))
  })
  const dynamoDB = new DynamoDB()
  const response = await dynamoDB.scaleUp()
  t.truthy(response.includes('The provisioned throughput for the table will not change'))
})

test.serial('scale up - Attempt to change a resource which is still in use', async t => {
  AWS.mock('DynamoDB', 'updateTable', (params, callback) => {
    callback(new Error('Attempt to change a resource which is still in use blah blah blah'))
  })
  const dynamoDB = new DynamoDB()
  const response = await dynamoDB.scaleUp()
  t.truthy(response.includes('Attempt to change a resource which is still in use'))
})

test.serial('scale up - undefined SCALE_UP_READ_CAPACITY', async t => {
  AWS.mock('DynamoDB', 'updateTable', (params, callback) => {
    callback(new Error('The provisioned throughput for the table will not change blah blah blah'))
  })
  const temp = process.env.SCALE_UP_READ_CAPACITY
  process.env.SCALE_UP_READ_CAPACITY = 'some string'
  const dynamoDB = new DynamoDB()
  const response = await dynamoDB.scaleUp()
  t.truthy(response.includes('The provisioned throughput for the table will not change'))
  process.env.SCALE_UP_READ_CAPACITY = temp
})

test.serial('scale up - Other error', async t => {
  AWS.mock('DynamoDB', 'updateTable', (params, callback) => {
    callback(new Error('Some other errors'))
  })
  const dynamoDB = new DynamoDB()
  try {
    await dynamoDB.scaleUp()
    t.fail('This case should throw an error')
  } catch (e) {
    t.is(e.message, 'Some other errors')
  }
})
