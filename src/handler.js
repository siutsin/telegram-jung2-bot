import Messages from './messages'
import OffFromWork from './offFromWork'
import SQS from './sqs'
import DynamoDB from './dynamodb'

const messages = new Messages()
const offFromWork = new OffFromWork()
const sqs = new SQS()
const dynamoDB = new DynamoDB()

export const onOffFromWork = async () => offFromWork.off()
export const onMessage = async (event) => messages.newMessage(event)
export const onEvent = async (event) => sqs.onEvent(event)
export const onScaleUp = async (event) => dynamoDB.scaleUp(event)
