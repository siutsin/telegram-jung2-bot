const moment = require('moment')
const AWS = require('aws-sdk')

require('dotenv').config({ path: '.env.development' })

const NUMBER_OF_USERS = 200
const NUMBER_OF_MESSAGES = 80000
const OFFSET_DATE_CREATED = -2
const TTL_DAYS = 90
const TABLE = ''
const BATCH_SIZE = 500

const credentials = new AWS.SharedIniFileCredentials({ profile: process.env.PROFILE })
AWS.config.credentials = credentials
AWS.config.update({ region: process.env.REGION })

const documentClient = new AWS.DynamoDB.DocumentClient()

async function saveStats ({ message, days = TTL_DAYS, i }) {
  const item = {
    chatId: message.chat.id,
    chatTitle: message.chat.title,
    userId: message.from.id,
    username: message.from.username,
    firstName: message.from.first_name,
    lastName: message.from.last_name,
    dateCreated: moment()
      .utcOffset(8)
      .add(OFFSET_DATE_CREATED, 'days')
      .subtract(i, 'seconds')
      .format(),
    ttl: moment()
      .utcOffset(8)
      .add(days, 'days')
      .unix()
  }
  return documentClient.put({ TableName: TABLE, Item: item }).promise()
}

async function main () {
  const userIds = new Array(NUMBER_OF_USERS).fill(1).map((_, i) => i + 1)
  let promiseArray = []
  let batchCount = 0
  for (let i = 0; i < NUMBER_OF_MESSAGES; i++) {
    const userId = userIds[Math.floor(Math.random() * userIds.length)]
    const message = {
      'message_id': 2,
      'from': {
        'id': userId,
        'is_bot': false,
        'first_name': `f${userId}`,
        'last_name': `l${userId}`,
        'username': `u${userId}`
      },
      'chat': {
        'id': -287173723, // test group id
        'title': 'Test2 jung2bot',
        'type': 'group',
        'all_members_are_administrators': true
      },
      'date': moment().unix(),
      'text': 'hi'
    }
    if (promiseArray.length < BATCH_SIZE) {
      promiseArray.push(saveStats({ message, i }))
    }
    if (promiseArray.length >= BATCH_SIZE) {
      await Promise.all(promiseArray)
      promiseArray.length = 0
      batchCount += BATCH_SIZE
      console.log('batchCount', batchCount)
    }
  }
  if (promiseArray.length > 0) {
    await Promise.all(promiseArray)
    batchCount += promiseArray.length
    console.log('batchCount', batchCount)
  }
}

main().catch(console.error)
