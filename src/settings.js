import Telegram from './telegram'
import DynamoDB from './dynamodb'

export default class Settings {
  constructor () {
    this.telegram = new Telegram()
    this.dynamodb = new DynamoDB()
  }

  async isAdmin ({ chatId, userId, allAdmin }) {
    let isAdmin = allAdmin
    if (!isAdmin) {
      isAdmin = await this.telegram.isAdmin({ chatId, userId })
    }
    return isAdmin
  }

  async enableAllJung ({ chatId, chatTitle, userId, allAdmin }) {
    if (await this.isAdmin({ chatId, userId, allAdmin })) {
      await this.dynamodb.enableAllJung({ chatId })
      await this.telegram.sendMessage(chatId, `圍爐區: ${chatTitle} - Enabled AllJung command`)
    }
  }

  async disableAllJung ({ chatId, chatTitle, userId, allAdmin }) {
    if (await this.isAdmin({ chatId, userId, allAdmin })) {
      await this.dynamodb.disableAllJung({ chatId })
      await this.telegram.sendMessage(chatId, `圍爐區: ${chatTitle} - Disabled AllJung command`)
    }
  }
}
