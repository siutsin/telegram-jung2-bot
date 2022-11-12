'use strict'

const fs = require('fs')
const path = require('path')
const moment = require('moment')

const util = require('util')
const writeFilePromise = util.promisify(fs.writeFile)
const accessPromise = util.promisify(fs.access)

const isMinify = false
const fullPath = path.resolve(__dirname, 'dynamodbMessagesSeed.json')

const data = []
const chatIds = new Array(1000).fill(1).map((_, i) => i + 1)
const userIds = new Array(10000).fill(1).map((_, i) => i + 1)

const generate = async () => {
  for (let i = 0; i < 20000; i++) {
    const chatId = chatIds[Math.floor(Math.random() * chatIds.length)]
    const userId = userIds[Math.floor(Math.random() * userIds.length)]
    const record = {
      chatId,
      chatTitle: `t${chatId}`,
      dateCreated: moment().utcOffset(8).subtract(i, 'minutes').format(),
      firstName: `f${userId}`,
      lastName: `l${userId}`,
      ttl: moment().add(7, 'days').unix(),
      userId,
      username: `u${userId}`
    }
    data.push(record)
  }

  try {
    await writeFilePromise(fullPath, JSON.stringify(data, null, isMinify ? 0 : 2))
    console.log(`JSON generated to ${fullPath}`)
  } catch (e) {
    console.error(e.message)
  }
}

accessPromise(fullPath, fs.constants.F_OK).then(() => {
  console.log(`${fullPath} exists, skipping...`)
  process.exit()
}).catch(async () => {
  console.log(`${fullPath} does not exist`)
  await generate()
})
