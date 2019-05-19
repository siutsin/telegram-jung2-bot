import axios from 'axios'
import Pino from 'pino'

export default class Telegram {
  constructor () {
    this.logger = new Pino({ level: process.env.LOG_LEVEL })
  }
  async sendMessage (chatId, message) {
    // TODO: TELEGRAM_BOT_TOKEN should be loaded from SSM Parameter Store
    const url = `https://api.telegram.org/bot${process.env.TELEGRAM_BOT_TOKEN}/sendMessage`
    const data = {
      chat_id: chatId,
      text: message
    }
    const response = await axios.post(url, data)
    this.logger.debug('response.status', response.status)
    this.logger.debug('response.data', response.data)
    return response
  }
}
