import test from 'ava'
import nock from 'nock'
import path from 'path'
import dotenv from 'dotenv'
import stubSaveMessageResponse from './stub/saveMessageResponse'
import Telegram from '../src/telegram'

dotenv.config({ path: path.resolve(__dirname, '.env.testing') })

test.afterEach.always(t => {
  nock.cleanAll()
})

test('sendMessage', async t => {
  nock(`https://api.telegram.org/bot${process.env.TELEGRAM_BOT_TOKEN}`)
    .post('/sendMessage')
    .reply(200, {
      data: stubSaveMessageResponse
    })
  const telegram = new Telegram()
  const response = await telegram.sendMessage(123, 'hihi')
  const data = response.data.data
  t.is(data.text, stubSaveMessageResponse.text)
})

test('sendMessage - failing - Telegram API returns HTTP 499 Error', async t => {
  nock(`https://api.telegram.org/bot${process.env.TELEGRAM_BOT_TOKEN}`)
    .post('/sendMessage')
    .reply(499)
  const telegram = new Telegram()
  const error = await t.throwsAsync(telegram.sendMessage(123, 'hihi'))
  t.is(error.message, 'Request failed with status code 499')
})
