'use strict'

const AWS = require('aws-sdk')
const copy = require('copy-dynamodb-table').copy
require('dotenv').config({ path: '.env.development' })

const sourceTable = ''

const credentials = new AWS.SharedIniFileCredentials({ profile: process.env.PROFILE })

const awsConfig = {
  region: process.env.REGION,
  accessKeyId: credentials.accessKeyId,
  secretAccessKey: credentials.secretAccessKey
}

copy({
  config: awsConfig,
  source: { tableName: sourceTable },
  destination: { tableName: process.env.MESSAGE_TABLE },
  log: true,
  create: false
},
function (err, result) {
  if (err) {
    console.error(err.message)
  }
  console.log(result)
})
