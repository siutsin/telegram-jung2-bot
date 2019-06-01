// https://aws.amazon.com/blogs/aws/new-manage-dynamodb-items-using-time-to-live-ttl/
// The attribute must be in DynamoDB’s Number data type, and is interpreted as seconds per the Unix Epoch time system.
// This is to convert the TTL from timestamp string to epoch time due to a mistake at the beginning

const AWS = require('aws-sdk')
const isString = require('lodash.isstring')
const moment = require('moment')

require('dotenv').config({ path: '.env.production' })

const credentials = new AWS.SharedIniFileCredentials({ profile: process.env.PROFILE })

AWS.config.credentials = credentials

AWS.config.update({
  region: process.env.REGION
})

const docClient = new AWS.DynamoDB.DocumentClient()

const hashKey = 'id'
const rangeKey = 'dateCreated'
const tableName = process.env.MESSAGE_TABLE

function buildKey (obj) {
  const key = {}
  key[hashKey] = obj[hashKey]
  if (rangeKey) {
    key[rangeKey] = obj[rangeKey]
  }
  return key
}

async function update (obj) {
  const newTTL = moment(obj.ttl).unix()
  console.log(`update ${obj.id} from ${obj.ttl} to ${newTTL}`)
  const params = {
    TableName: tableName,
    Key: buildKey(obj),
    ReturnValues: 'NONE', // optional (NONE | ALL_OLD)
    ReturnConsumedCapacity: 'NONE', // optional (NONE | TOTAL | INDEXES)
    ReturnItemCollectionMetrics: 'NONE', // optional (NONE | SIZE)
    UpdateExpression: 'set #ttl = :ttl',
    ExpressionAttributeNames: {
      '#ttl': 'ttl'
    },
    ExpressionAttributeValues: {
      ':ttl': newTTL
    }
  }
  return docClient.update(params).promise()
}

async function scanAll (startKey) {
  const scanParams = {
    TableName: tableName
    // TableName: tableName,
    // FilterExpression: 'contains(#ttl, :ttl)',
    // ExpressionAttributeNames: {
    //   '#ttl': 'ttl'
    // },
    // ExpressionAttributeValues: {
    //   ':ttl': { 'S': '2019-0' }
    // }
  }
  if (startKey) {
    scanParams.ExclusiveStartKey = startKey
  }
  // console.log(scanParams)
  const data = await docClient.scan(scanParams).promise()
  console.log('Count', data.Count, 'ScannedCount', data.ScannedCount, 'startKey', startKey)
  let promiseArray = []
  for (let i = 0; i < data.Items.length; i++) {
    const obj = data.Items[i]
    if (isString(obj.ttl)) {
      if (promiseArray.length < 15) {
        promiseArray.push(update(obj))
      }
      if (promiseArray.length >= 15) {
        await Promise.all(promiseArray)
        promiseArray.length = 0
      }
    }
  }
  if (promiseArray.length > 0) {
    await Promise.all(promiseArray)
  }
  return data.LastEvaluatedKey
}

async function main () {
  let lastEvaluatedKey = false
  let i = 0
  do {
    console.log(`i: ${i} lastEvaluatedKey: ${JSON.stringify(lastEvaluatedKey)}`)
    lastEvaluatedKey = await scanAll(lastEvaluatedKey)
    i++
  } while (lastEvaluatedKey)
}

main().catch(console.error)
