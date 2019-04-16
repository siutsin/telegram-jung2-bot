import Messages from './messages'
import OffFromWork from './offFromWork'

const messages = new Messages()
const offFromWork = new OffFromWork()

export const onOffFromWork = async () => offFromWork.off()
export const onMessage = async (event) => messages.newMessage(event)
