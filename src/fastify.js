require('dotenv').config({ path: '.env.development' })
const fastify = require('fastify')({ logger: true })
const handler = require('./handler')

const toEventObject = (request) => {
  // dummy telegram ip address
  const dummyHeader = { 'X-Forwarded-For': '91.108.4.0' }

  return {
    headers: process.env.STAGE === 'dev' ? { ...dummyHeader, ...request.headers } : request.headers,
    body: JSON.stringify(request.body)
  }
}

fastify.route({
  method: 'GET',
  url: `/jung2bot/${process.env.STAGE}/ping`,
  handler: async (request, reply) => {
    return { health: 'ok' }
  }
})

fastify.route({
  method: 'POST',
  url: `/jung2bot/${process.env.STAGE}/`,
  handler: async (request, reply) => {
    const response = await handler.onMessage(toEventObject(request))
    reply.code(response.statusCode)
    return response
  }
})

fastify.route({
  method: 'POST',
  url: `/jung2bot/${process.env.STAGE}/onEvent`,
  handler: async (request, reply) => {
    const response = await handler.onEvent(toEventObject(request))
    reply.code(response.statusCode)
    return response
  }
})

fastify.route({
  method: 'POST',
  url: `/jung2bot/${process.env.STAGE}/onScaleUp`,
  handler: async (request) => {
    return handler.onScaleUp(toEventObject(request))
  }
})

fastify.route({
  method: 'GET',
  url: `/jung2bot/${process.env.STAGE}/onOffFromWork`,
  handler: async (request) => {
    return handler.onOffFromWork()
  }
})

const start = async () => {
  try {
    const ip = process.env.DOCKER ? '0.0.0.0' : '127.0.0.1'
    await fastify.listen(3000, ip)
  } catch (err) {
    fastify.log.error(err)
    process.exit(1)
  }
}
start().catch(r => fastify.log.error(r))
