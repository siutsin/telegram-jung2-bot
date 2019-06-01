import Messages from './messages'
import OffFromWork from './offFromWork'
import SQS from './sqs'
import DynamoDB from './dynamodb'

const messages = new Messages()
const offFromWork = new OffFromWork()
const sqs = new SQS()
const dynamoDB = new DynamoDB()

// noinspection JSUnusedGlobalSymbols
export const onOffFromWork = async () => offFromWork.off()
// noinspection JSUnusedGlobalSymbols
export const onMessage = async (event) => messages.newMessage(event)
// noinspection JSUnusedGlobalSymbols
export const onEvent = async (event) => sqs.onEvent(event)
// noinspection JSUnusedGlobalSymbols
export const onScaleUp = async (event) => dynamoDB.scaleUp(event)
