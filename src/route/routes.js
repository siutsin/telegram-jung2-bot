import Root from './root'
import Bot from './bot'

export default class Routes {

  constructor (app, bot) {
    this.app = app
    this.bot = bot
    this.rootRoute = new Root()
    this.botRoute = new Bot(this.bot)
  }

  configRoutes () {
    this.app.get('/', this.rootRoute.root())
    this.app.post(`/bot${process.env.TELEGRAM_BOT_TOKEN}`, this.botRoute.root(this.bot))
  }
}
