import express from 'express'
import bodyParser from 'body-parser'
import morgan from 'morgan'
import TelegramBot from 'node-telegram-bot-api'
import log from 'log-to-file-and-console-node'

import CronController from './controller/cron'
import BotHandler from './botHandler'
import Routes from './route/routes'

/**
 * HTTP Server
 */
const app = express()
app.use(bodyParser.json())
app.use(morgan('combined', {'stream': log.stream}))

/**
 * Bot
 */
const bot = new TelegramBot(process.env.TELEGRAM_BOT_TOKEN, {polling: true})
const botHandler = new BotHandler(bot)
bot.onText(/\/top(t|T)en/, msg => botHandler.onTopTen(msg))
bot.onText(/\/all(j|J)ung/, msg => botHandler.onAllJung(msg))
bot.onText(/\/jung(h|H)elp/, msg => botHandler.onHelp(msg))
bot.onText(/\/debug/, msg => botHandler.onDebug(msg))
bot.on('message', msg => botHandler.onMessage(msg))
bot.on('polling_error', error => log.e(`polling_error: ${JSON.stringify(error)}`))

/**
 * Routing
 */
const routes = new Routes(app, bot)
routes.configRoutes(bot)

/**
 * Cron Jobs
 */
const cron = new CronController(bot)
cron.databaseMaintenance()
cron.startAllCronJobs()

export default app
