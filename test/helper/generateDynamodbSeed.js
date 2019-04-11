'use strict'

const fs = require('fs')
const path = require('path')
const uuid = require('uuid')
const moment = require('moment')

const util = require('util')
const writeFilePromise = util.promisify(fs.writeFile)
const accessPromise = util.promisify(fs.access)

const isMinify = true
const fullPath = path.resolve(__dirname, 'dynamodbSeed.json')

const data = []
const chatIds = [-5, -4, -3, -2, -1, 1, 2, 3, 4, 5]
const userIds = [123, 234, 345, 456, 567, 678, 789, 1234, 2345, 3456, 4567, 5678, 6789, 7890, 12345, 23456]

const generate = async () => {
  for (let i = 0; i < 10000; i++) {
    const dateCreated = moment().utcOffset(8).subtract(i, 'minutes')
    const ttl = moment().add(7, 'days')
    const chatId = chatIds[Math.floor(Math.random() * chatIds.length)]
    const userId = userIds[Math.floor(Math.random() * userIds.length)]
    let record = {
      chatId: chatId,
      chatTitle: `chatTitle ${chatId}`,
      dateCreated: dateCreated.format(),
      firstName: `firstName${userId}`,
      lastName: `lastName${userId}`,
      id: uuid.v4(),
      ttl: ttl.format(),
      userId: userId,
      username: `username${userId}`
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
