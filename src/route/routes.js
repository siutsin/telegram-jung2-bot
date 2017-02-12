import Root from './root'

class Routes {

  constructor (app) {
    this.app = app
  }

  configRoutes () {
    this.app.get('/', Root.root())
  }
}

export default Routes
