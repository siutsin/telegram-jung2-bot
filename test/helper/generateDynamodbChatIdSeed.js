'use strict'

const fs = require('fs')
const path = require('path')
const moment = require('moment')

const util = require('util')
const writeFilePromise = util.promisify(fs.writeFile)
const accessPromise = util.promisify(fs.access)

const isMinify = false
const fullPath = path.resolve(__dirname, 'dynamodbChatIdSeed.json')

const data = []

const generate = async () => {
  for (let i = 0; i < 110; i++) {
    let record = {
      chatId: i,
      dateCreated: moment().utcOffset(8).subtract(i, 'minutes').format(),
      ttl: moment().add(7, 'days').unix()
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
