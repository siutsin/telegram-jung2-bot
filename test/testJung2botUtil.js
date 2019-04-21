import test from 'ava'
import nock from 'nock'
import path from 'path'
import dotenv from 'dotenv'
import stubSaveMessageResponse from './stub/saveMessageResponse'
import Jung2botUtil from '../src/jung2botUtil'

dotenv.config({ path: path.resolve(__dirname, '.env.testing') })

const jung2botUtil = new Jung2botUtil()

test('sendMessage', async t => {
  nock(`https://api.telegram.org/bot${process.env.TELEGRAM_BOT_TOKEN}`)
    .post('/sendMessage')
    .reply(200, {
      data: stubSaveMessageResponse
    })
  const response = await jung2botUtil.sendMessage(123, 'hihi')
  const data = response.data.data
  t.is(data.text, stubSaveMessageResponse.text)
  nock.restore()
})

test('sendMessage - failing - Telegram API returns HTTP 400 Error', async t => {
  nock(`https://api.telegram.org/bot${process.env.TELEGRAM_BOT_TOKEN}`)
    .post('/sendMessage')
    .reply(400)
  const error = await t.throwsAsync(jung2botUtil.sendMessage(123, 'hihi'))
  t.is(error.message, 'Request failed with status code 400')
  nock.restore()
})
