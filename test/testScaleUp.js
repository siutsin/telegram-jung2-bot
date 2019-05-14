import test from 'ava'
import path from 'path'
import dotenv from 'dotenv'
import AWS from 'aws-sdk-mock'
import DynamoDB from '../src/dynamodb'

dotenv.config({ path: path.resolve(__dirname, '.env.testing') })

test.beforeEach(t => {
  AWS.mock('DynamoDB', 'describeTable', (params, callback) => {
    callback(null, {
      Table: {
        ProvisionedThroughput: {
          WriteCapacityUnits: 1
        }
      }
    })
  })
})

test.afterEach.always(t => {
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
