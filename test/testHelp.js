import test from 'ava'
import nock from 'nock'
import path from 'path'
import stubHelpMessage from './stub/helpMessage'
import stubHelpMessageResponse from './stub/helpMessageResponse'
import Help from '../src/help'
import dotenv from 'dotenv'
dotenv.config({ path: path.resolve(__dirname, '.env.testing') })

const help = new Help()

test('sendHelpMessage', async t => {
  nock(`https://api.telegram.org/bot${process.env.TELEGRAM_BOT_TOKEN}`)
    .post('/sendMessage')
    .reply(200, {
      data: stubHelpMessageResponse
    })
  const response = await help.sendHelpMessage(stubHelpMessage)
  t.is(response.data.data.result.text, stubHelpMessageResponse.result.text)
  nock.restore()
})
