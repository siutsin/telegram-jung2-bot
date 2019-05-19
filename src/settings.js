import Telegram from './telegram'
import DynamoDB from './dynamodb'

export default class Settings {
  constructor () {
    this.telegram = new Telegram()
    this.dynamodb = new DynamoDB()
  }

  async enableAllJung ({ chatId, userId }) {
    const isAdmin = await this.telegram.isAdmin({ chatId, userId })
    let resultMessage = ''
    if (isAdmin) {
      await this.dynamodb.enableAllJung({ chatId })
    }
    return this.telegram.sendMessage(chatId, resultMessage)
  }

  async disableAllJung ({ chatId, userId }) {
    const isAdmin = await this.telegram.isAdmin({ chatId, userId })
    let resultMessage = ''
    if (isAdmin) {
      await this.dynamodb.disableAllJung({ chatId })
    }
    return this.telegram.sendMessage(chatId, resultMessage)
  }
}
