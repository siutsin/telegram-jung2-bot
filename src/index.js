/*
  This is a hacky fastify server to migrate from Serverless to ECS due to the cost issue.
  Some choices here might not make any sense, but I do not intend to implement significant
  enhancements in this version.
 */

require('dotenv').config({ path: '.env.development' })

const AWS = require('aws-sdk')
const Pino = require('pino')
const fastify = require('fastify')({ logger: { level: process.env.LOG_LEVEL }, trustProxy: true })
const https = require('https')
const ip = require('ip')
const { Consumer } = require('sqs-consumer')
const { performance } = require('perf_hooks')

const SQS = require('./sqs.js')
const handler = require('./handler')

const logger = new Pino({ level: process.env.LOG_LEVEL })
const sqs = new SQS()

// SQS Consumer. Use long polling to receive SQS messages..

const toSQSLambdaEvent = (message) => {
  return {
    Records: [
      {
        receiptHandle: message.ReceiptHandle,
        messageAttributes: message.MessageAttributes
      }
    ]
  }
}

const consumer = Consumer.create({
  queueUrl: process.env.EVENT_QUEUE_URL,
  batchSize: 10, // aws max 10
  messageAttributeNames: ['chatId', 'chatTitle', 'userId', 'action', 'offTime', 'workday', 'timeString'],
  handleMessageBatch: async (messages) => {
    const startTime = performance.now()
    const requests = []
    for (const message of messages) {
      requests.push(sqs.onEvent(toSQSLambdaEvent(message)))
    }
    await Promise.all(requests)
    const endTime = performance.now()
    logger.warn(`handleMessageBatch time: ${endTime - startTime} ms`)
  },
  sqs: new AWS.SQS({
    httpOptions: {
      agent: new https.Agent({
        keepAlive: true
      })
    }
  })
})
consumer.on('error', (err) => {
  logger.error(`sqs-consumer error: ${err.message}. Exit 1`)
  process.exit(1)
})
consumer.on('processing_error', (err) => {
  logger.error(`sqs-consumer processing_error: ${err.message}`)
})
consumer.start()

// fastify

const toEventObject = (request) => {
  // dummy telegram ip address
  const dummyHeader = { 'X-Forwarded-For': '91.108.4.0' }

  return {
    headers: process.env.STAGE === 'dev' ? { ...dummyHeader, ...request.headers } : request.headers,
    body: JSON.stringify(request.body)
  }
}

const isTelegramIP = (requestIP) => {
  if (!ip.cidrSubnet('91.108.4.0/22').contains(requestIP) && !ip.cidrSubnet('149.154.160.0/20').contains(requestIP)) {
    throw new Error('Not Telegram IP')
  }
  return true
}

const staticFunctionHandler = async (request, reply, functionName) => {
  try {
    if (!process.env.DOCKER) {
      fastify.log.info(request.ips[0])
      isTelegramIP(request.ips[0])
    }
    await handler[functionName]()
    reply.code(200)
    return { [functionName]: 'ok' }
  } catch (err) {
    fastify.log.error(err)
    reply.code(503)
    return { [functionName]: 'failed' }
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
  method: 'GET',
  url: `/jung2bot/${process.env.STAGE}/onScaleUp`,
  handler: async (request, reply) => {
    return staticFunctionHandler(request, reply, 'onScaleUp')
  }
})

fastify.route({
  method: 'GET',
  url: `/jung2bot/${process.env.STAGE}/onOffFromWork`,
  handler: async (request, reply) => {
    const timeString = request.query.timeString
    await handler.onOffFromWork(timeString)
    reply.code(202)
    return { onOffFromWork: 'ok' }
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
