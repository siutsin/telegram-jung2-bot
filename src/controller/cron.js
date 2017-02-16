require('babel-polyfill')

import _ from 'lodash'
import { CronJob } from 'cron'
import async from 'async'

import MessageController from './message'
import UsageController from './usage'
import c from '../constants'

const messageController = new MessageController()
const usageController = new UsageController()

export default class DebugController {

  constructor (bot) {
    this.bot = bot
  }

  databaseMaintenance () {
    messageController.cleanup()
    usageController.cleanup()
  }

  offJob () {
    return new CronJob({
      cronTime: c.CRON.OFF_JOB_PATTERN,
      onTick: async () => {
        const chatIds = await messageController.getAllGroupIds()
        async.each(chatIds, async chatId => {
          const msg = {chat: {id: chatId}}
          this.bot.sendMessage(chatId, c.CRON.OFF_JOB)
          const message = await messageController.getTopTen(msg, true)
          if (!_.isEmpty(message)) { this.bot.sendMessage(chatId, message) }
        })
      },
      timeZone: c.CONFIG.TIMEZONE
    })
  }

  cleanupJob () {
    return new CronJob({
      cronTime: c.CRON.DB_CLEANUP_PATTERN,
      onTick: () => this.databaseMaintenance(),
      timeZone: c.CONFIG.TIMEZONE
    })
  }

  startAllCronJobs () {
    const offJob = this.offJob()
    const cleanupJob = this.cleanupJob()
    offJob.start()
    cleanupJob.start()
  }
}
