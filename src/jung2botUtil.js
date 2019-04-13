import axios from 'axios'
// import Pino from 'pino'

export default class jung2botUtil {
  constructor () {
    // this.logger = new Pino({ level: process.env.LOG_LEVEL })
    // const that = this
    axios.defaults.headers.common['Content-Type'] = 'application/json'
    // axios.interceptors.request.use(config => {
    //   that.logger.debug('interceptors config', config)
    //   return config
    // }, error => {
    //   that.logger.error('error', error)
    //   return Promise.reject(error)
    // })
    // axios.interceptors.response.use(response => {
    //   that.logger.debug('interceptors response', response)
    //   return response
    // }, error => {
    //   that.logger.error('error', error)
    //   return Promise.reject(error)
    // })
  }

  async sendMessage (chatId, message) {
    // TODO: TELEGRAM_BOT_TOKEN should be loaded from SSM Parameter Store
    const url = `https://api.telegram.org/bot${process.env.TELEGRAM_BOT_TOKEN}/sendMessage`
    const data = {
      chat_id: chatId,
      text: message
    }
    // this.logger.debug('data', data)
    const response = await axios.post(url, data)
    // this.logger.debug('response', response)
    return response
  }
}
