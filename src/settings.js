import Telegram from './telegram'
import DynamoDB from './dynamodb'

export default class Settings {
  constructor () {
    this.telegram = new Telegram()
    this.dynamodb = new DynamoDB()
  }

  async enableAllJung ({ chatId, userId }) {
    const isAdmin = await this.telegram.isAdmin({ chatId, userId })
    if (isAdmin) {
      await this.dynamodb.enableAllJung({ chatId })
      await this.telegram.sendMessage(chatId, 'Enabled AllJung command')
    }
  }

  async disableAllJung ({ chatId, userId }) {
    const isAdmin = await this.telegram.isAdmin({ chatId, userId })
    if (isAdmin) {
      await this.dynamodb.disableAllJung({ chatId })
      await this.telegram.sendMessage(chatId, 'Disabled AllJung command')
    }
  }
}
