import 'codecov'
import log from 'log-to-file-and-console-node'
log.removeConsole()
process.env.MONGODB_URL = '127.0.0.1:27017/jung2botTest'
process.env.MONGODB_CACHE_DO_URL = '127.0.0.1:27017/jung2botTestCache'

// TODO:
