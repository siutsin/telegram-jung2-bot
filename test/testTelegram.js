import test from 'ava'
import nock from 'nock'
import path from 'path'
import dotenv from 'dotenv'
import stubSaveMessageResponse from './stub/saveMessageResponse'
import stubGetChatAdministratorsResponse from './stub/getChatAdministratorsResponse'
import Telegram from '../src/telegram'

dotenv.config({ path: path.resolve(__dirname, '.env.testing') })

test.afterEach.always(() => {
  nock.cleanAll()
})

test('sendMessage', async t => {
  nock(`https://api.telegram.org/bot${process.env.TELEGRAM_BOT_TOKEN}`)
    .post('/sendMessage')
    .reply(200, {
      data: stubSaveMessageResponse
    })
  const telegram = new Telegram()
  const response = await telegram.sendMessage(123, 'hi')
  const data = response.data.data
  t.is(data.text, stubSaveMessageResponse['text'])
})

test('sendMessage - failing - Telegram API returns HTTP 499 Error', async t => {
  nock(`https://api.telegram.org/bot${process.env.TELEGRAM_BOT_TOKEN}`)
    .post('/sendMessage')
    .reply(499)
  const telegram = new Telegram()
  const error = await t.throwsAsync(telegram.sendMessage(123, 'hi'))
  t.is(error.message, 'Request failed with status code 499')
})

test('isAdmin - true', async t => {
  nock(`https://api.telegram.org/bot${process.env.TELEGRAM_BOT_TOKEN}`)
    .get('/getChatAdministrators')
    .query({ chat_id: 123 })
    .reply(200, stubGetChatAdministratorsResponse)
  const telegram = new Telegram()
  const response = await telegram.isAdmin({ chatId: 123, userId: 234 })
  t.truthy(response)
})

test('isAdmin - false', async t => {
  nock(`https://api.telegram.org/bot${process.env.TELEGRAM_BOT_TOKEN}`)
    .get('/getChatAdministrators')
    .query({ chat_id: 123 })
    .reply(200, stubGetChatAdministratorsResponse)
  const telegram = new Telegram()
  const response = await telegram.isAdmin({ chatId: 123, userId: 999 })
  t.falsy(response)
})
