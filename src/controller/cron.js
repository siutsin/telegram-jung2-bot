require('babel-polyfill')

import _ from 'lodash'
import {CronJob} from 'cron'
import async from 'async'

import MessageController from './messageFacade'
import UsageController from './usage'

const usageController = new UsageController()

export default class DebugController {

  constructor (bot) {
    this.bot = bot
  }

  databaseMaintenance () {
    MessageController.cleanup()
    usageController.cleanup()
  }

  offJob () {
    return new CronJob({
      cronTime: '00 00 18 * * 1-5',
      onTick: async () => {
        const chatIds = await MessageController.getAllGroupIds()
        async.each(chatIds, async chatId => {
          const msg = { chat: { id: chatId } }
          this.bot.sendMessage(chatId, '夠鐘收工~~')
          const message = await MessageController.getTopTen(msg, true)
          if (!_.isEmpty(message)) { this.bot.sendMessage(chatId, message) }
        })
      },
      timeZone: 'Asia/Hong_Kong'
    })
  }

  cleanupJob () {
    return new CronJob({
      cronTime: '0 0 0-17,19-23 * * *',
      onTick: () => this.databaseMaintenance(),
      timeZone: 'Asia/Hong_Kong'
    })
  }

  startAllCronJobs () {
    const offJob = this.offJob()
    const cleanupJob = this.cleanupJob()
    offJob.start()
    cleanupJob.start()
  }
}
