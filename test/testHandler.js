import test from 'ava'
import path from 'path'
import dotenv from 'dotenv'

dotenv.config({ path: path.resolve(__dirname, '.env.testing') })
const handler = require('../src/handler')

test.failing('handler.onMessage()', async t => {
  const event = {} // TODO: get event from CloudWatch
  const response = await handler.onMessage(event)
  t.is(response, '')
})
