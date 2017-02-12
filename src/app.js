import express from 'express'
import bodyParser from 'body-parser'
import morgan from 'morgan'
import TelegramBot from 'node-telegram-bot-api'
import log from 'log-to-file-and-console-node'

import MessageController from './controller/messageFacade'
import CronController from './controller/cron'
import DebugController from './controller/debug'
import BotHandler from './route/botHandler'
import SystemAdmin from './helper/SystemAdmin'
import root from './route/root'

const app = express()
const bot = new TelegramBot(process.env.TELEGRAM_BOT_TOKEN, { polling: true })
const botHandler = new BotHandler(bot)
const systemAdmin = new SystemAdmin()

MessageController.init() // TODO: remove

app.use(morgan('combined', { 'stream': log.stream }))
app.use(bodyParser.json())

app.use('/', root)

bot.onText(/\/top(t|T)en/, msg => botHandler.onTopTen(msg))
bot.onText(/\/all(j|J)ung/, msg => botHandler.onAllJung(msg))
bot.onText(/\/jung(h|H)elp/, msg => botHandler.onHelp(msg))
bot.on('message', msg => botHandler.onMessage(msg))

bot.onText(/\/debug/, msg => {
  if (systemAdmin.isAdmin(msg)) {
    const debugController = new DebugController(bot)
    debugController.healthCheck(msg)
  }
})

const cron = new CronController(bot)
cron.startAllCronJobs()
// cleanup when service start
cron.databaseMaintenance()

export default app
