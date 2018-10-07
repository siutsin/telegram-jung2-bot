#!/usr/bin/env node

/**
 * Module dependencies.
 */
import '../env'
import app from '../app'
import log from 'log-to-file-and-console-node'
import http from 'http'

/**
 * Normalize a port into a number, string, or false.
 */
const normalizePort = val => {
  const port = parseInt(val, 10)
  if (isNaN(port)) { return val }
  if (port >= 0) { return port }
  return false
}

/**
 * Get port from environment and store in Express.
 */
const port = normalizePort(process.env.PORT || '3000')
app.set('port', port)

/**
 * Create HTTP server.
 */
const server = http.createServer(app)

/**
 * Listen on provided port, on all network interfaces.
 */
server.listen(port, () => {
  log.i(`Express server is listening on ${port}`)
})
server.on('error', error => {
  if (error.syscall !== 'listen') { throw error }
  const bind = typeof port === 'string' ? `Pipe ${port}` : `Port ${port}`
  // handle specific listen errors with friendly messages
  switch (error.code) {
    case 'EACCES':
      log.e(`${bind} requires elevated privileges`)
      process.exit(1)
      break
    case 'EADDRINUSE':
      log.e(`${bind} is already in use`)
      process.exit(1)
      break
    default:
      throw error
  }
})
