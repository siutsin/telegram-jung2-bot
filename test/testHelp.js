import test from 'ava'
import nock from 'nock'
import path from 'path'
import stubHelpMessageResponse from './stub/helpMessageResponse'
import Help from '../src/help'
import dotenv from 'dotenv'

dotenv.config({ path: path.resolve(__dirname, '.env.testing') })

test.afterEach.always(t => {
  nock.cleanAll()
})

test('sendHelpMessage', async t => {
  nock(`https://api.telegram.org/bot${process.env.TELEGRAM_BOT_TOKEN}`)
    .persist()
    .post('/sendMessage')
    .reply(200, {
      data: stubHelpMessageResponse
    })
  const help = new Help()
  const response = await help.sendHelpMessage(123)
  t.is(response.data.data.result.text, stubHelpMessageResponse.result.text)
})
