import Telegram from './telegram'

export default class Settings {
  constructor () {
    this.telegram = new Telegram()
  }

  async enableAllJung ({ chatId, userId }) {
    const isAdmin = await this.telegram.isAdmin({ chatId, userId })
    let resultMessage = ''
    if (isAdmin) {
      // save db
    }
    return this.telegram.sendMessage(chatId, resultMessage)
  }

  async disableAllJung ({ chatId, userId }) {
    const isAdmin = await this.telegram.isAdmin({ chatId, userId })
    let resultMessage = ''
    if (isAdmin) {
      // save db
    }
    return this.telegram.sendMessage(chatId, resultMessage)
  }
}
