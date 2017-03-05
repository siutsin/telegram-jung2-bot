import Root from './root'

export default class Routes {
  constructor (app, bot) {
    this.app = app
    this.bot = bot
    this.rootRoute = new Root()
  }

  configRoutes () {
    this.app.get('/', this.rootRoute.root())
  }
}
