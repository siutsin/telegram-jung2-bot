import test from 'ava'
import path from 'path'
import dotenv from 'dotenv'
import Messages from '../src/messages'

dotenv.config({ path: path.resolve(__dirname, '.env.testing') })

const messages = new Messages()

test.failing('newMessage', async t => {
  t.is(messages.toString(), '')
})
