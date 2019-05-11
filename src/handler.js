import Messages from './messages'
import OffFromWork from './offFromWork'
import SQS from './sqs'

const messages = new Messages()
const offFromWork = new OffFromWork()
const sqs = new SQS()

export const onOffFromWork = async () => offFromWork.off()
export const onMessage = async (event) => messages.newMessage(event)
export const onAllJung = async (event) => sqs.onEvent(event)
