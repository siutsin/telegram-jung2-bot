'use strict'

const fs = require('fs')
const path = require('path')
const moment = require('moment')

const stubAllJungDBResponse = require('../stub/allJungDatabaseResponse')

const util = require('util')
const writeFilePromise = util.promisify(fs.writeFile)
const accessPromise = util.promisify(fs.access)

const isMinify = false
const fullPath = path.resolve(__dirname, 'allJungDatabaseResponse.json')

/**
 * Returns a random integer between min (inclusive) and max (inclusive).
 * The value is no lower than min (or the next integer greater than min
 * if min isn't an integer) and no greater than max (or the next integer
 * lower than max if max isn't an integer).
 * Using Math.round() will give you a non-uniform distribution!
 */
function getRandomInt (min, max) {
  min = Math.ceil(min)
  max = Math.floor(max)
  return Math.floor(Math.random() * (max - min + 1)) + min
}

function shuffleArray (array) {
  for (let i = array.length - 1; i > 0; i--) {
    const j = Math.floor(Math.random() * (i + 1))
    const temp = array[i]
    array[i] = array[j]
    array[j] = temp
  }
}

const generate = async () => {
  stubAllJungDBResponse.Items.forEach(o => {
    const randomNumber = getRandomInt(1, 365)
    o.dateCreated = moment().subtract(randomNumber, 'days').format()
  })
  shuffleArray(stubAllJungDBResponse.Items)
  try {
    await writeFilePromise(fullPath, JSON.stringify(stubAllJungDBResponse, null, isMinify ? 0 : 2))
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
