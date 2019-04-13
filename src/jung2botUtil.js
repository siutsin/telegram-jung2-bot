import axios from 'axios'
import Pino from 'pino'

export default class jung2botUtil {
  constructor () {
    this.logger = new Pino({ level: process.env.LOG_LEVEL })
    const that = this
    axios.defaults.headers.post['Content-Type'] = 'application/json'
    axios.interceptors.request.use(config => {
      // this.logger.debug('interceptors config.header', config.headers)
      // this.logger.debug('interceptors config.data', config.data)
      return config
    }, error => {
      that.logger.error('error', error)
      return Promise.reject(error)
    })
    axios.interceptors.response.use(response => {
      this.logger.debug('interceptors response.status', response.status)
      this.logger.debug('interceptors response.header', response.headers)
      this.logger.debug('interceptors response.data', response.data)
      // this.logger.debug('response', response)
      return response
    }, error => {
      that.logger.error('error', error)
      return Promise.reject(error)
    })
  }

  async sendMessage (chatId, message) {
    // TODO: TELEGRAM_BOT_TOKEN should be loaded from SSM Parameter Store
    const url = `https://api.telegram.org/bot${process.env.TELEGRAM_BOT_TOKEN}/sendMessage`
    const data = {
      chat_id: chatId,
      text: message
    }
    // this.logger.debug('data', data)
    const response = await axios({
      method: 'post',
      responseType: 'json',
      headers: { 'Content-Type': 'application/json' },
      url,
      data
    })
    // this.logger.debug('response.status', response.status)
    // this.logger.debug('response.header', response.headers)
    // this.logger.debug('response', response)
    return response
  }
}
