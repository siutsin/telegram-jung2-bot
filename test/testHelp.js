import test from 'ava'
import nock from 'nock'
import path from 'path'
import stubHelpMessageResponse from './stub/helpMessageResponse'
import Help from '../src/help'
import dotenv from 'dotenv'

dotenv.config({ path: path.resolve(__dirname, '.env.testing') })

test.afterEach.always(() => {
  nock.cleanAll()
})

test('sendHelpMessage', async t => {
  nock(`https://api.telegram.org/bot${process.env.TELEGRAM_BOT_TOKEN}`)
    .persist()
    .post('/sendMessage')
    .reply(200, stubHelpMessageResponse)
  const help = new Help()
  const response = await help.sendHelpMessage({ chatId: 123, chatTitle: 'someTitle' })
  t.regex(response, /圍爐區: someTitle/)
  t.regex(response, /冗員\[jung2jyun4] Excess personnel in Cantonese/)
})
