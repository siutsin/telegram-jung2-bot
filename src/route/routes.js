import Root from './root'

export default class Routes {

  constructor (app) {
    this.app = app
  }

  configRoutes () {
    this.app.get('/', Root.root())
  }
}
