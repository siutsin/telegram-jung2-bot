import Messages from './messages'

const messages = new Messages()

export const onMessage = async (event) => messages.newMessage(event)
