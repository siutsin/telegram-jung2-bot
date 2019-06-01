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

  async enableAllJung ({ chatId, userId, allAdmin }) {
    if (await this.isAdmin({ chatId, userId, allAdmin })) {
      await this.dynamodb.enableAllJung({ chatId })
      await this.telegram.sendMessage(chatId, 'Enabled AllJung command')
    }
  }

  async disableAllJung ({ chatId, userId, allAdmin }) {
    if (await this.isAdmin({ chatId, userId, allAdmin })) {
      await this.dynamodb.disableAllJung({ chatId })
      await this.telegram.sendMessage(chatId, 'Disabled AllJung command')
    }
  }
}
