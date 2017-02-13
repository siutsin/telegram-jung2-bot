import log from 'log-to-file-and-console-node'
import _ from 'lodash'
import MessageController from '../controller/messageFacade'
import HelpController from '../controller/help'
import DebugController from '../controller/debug'
import SystemAdmin from '../helper/systemAdmin'

const helpController = new HelpController()
const systemAdmin = new SystemAdmin()

export default class BotHandler {

  constructor (bot) {
    this.bot = bot
  }

  async onTopTen (msg) {
    try {
      const message = await MessageController.getTopTen(msg)
      if (!_.isEmpty(message)) { this.bot.sendMessage(msg.chat.id, message) }
    } catch (e) {
      log.e('/topten err: ' + e.message, process.env.DISABLE_LOGGING)
      this.bot.sendMessage(msg.chat.id, 'Server Error')
    }
  }

  async onAllJung (msg) {
    log.i('/alljung msg: ' + JSON.stringify(msg), process.env.DISABLE_LOGGING)
    try {
      const message = await MessageController.getAllJung(msg)
      if (!_.isEmpty(message)) { this.bot.sendMessage(msg.chat.id, message) }
    } catch (e) {
      log.e('/alljung err: ' + e.message, process.env.DISABLE_LOGGING)
      this.bot.sendMessage(msg.chat.id, 'Server Error')
    }
  }

  onHelp (msg) {
    this.bot.sendMessage(msg.chat.id, helpController.getHelp())
  }

  onMessage (msg) {
    log.i('msg: ' + JSON.stringify(msg), process.env.DISABLE_LOGGING)
    if (MessageController.shouldAddMessage(msg)) {
      MessageController.addMessage(msg, () => log.i('add message success', process.env.DISABLE_LOGGING))
    } else {
      log.e('skip repeated message', process.env.DISABLE_LOGGING)
    }
  }

  onDebug (msg) {
    if (systemAdmin.isAdmin(msg)) {
      const debugController = new DebugController(this.bot)
      debugController.healthCheck(msg)
    }
  }

}
