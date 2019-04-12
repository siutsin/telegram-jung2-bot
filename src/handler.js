import DynamoDB from './dynamodb'
import Messages from './messages'

const dynamodb = new DynamoDB()
const messages = new Messages({ dynamodb })

export const onMessage = async (event) => messages.newMessage(event)
