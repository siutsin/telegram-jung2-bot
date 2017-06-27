import '../../src/env'
import log from 'log-to-file-and-console-node'
import mongoose from 'mongoose'
import faker from 'faker'
import MessageController from '../../src/controller/message'
import Message from '../../src/model/message'

const messageController = new MessageController()

const NO_OF_USER = 20

let users = []
for (let i = 0; i < NO_OF_USER; i++) {
  users.push({
    id: `id-${i}`,
    username: faker.internet.userName(),
    first_name: faker.name.firstName(),
    last_name: faker.name.lastName()
  })
}
// make some repetition
for (let i = 0; i < NO_OF_USER; i++) {
  for (let j = 0; j < i; j++) {
    users.push(users[i])
  }
}

mongoose.connect(process.env.MONGODB_CACHE_DO_URL, {db: {nativeParser: true}})

const populate = async () => {
  await Message.remove({})
  let batch = 10
  let total = 1e6
  let milestone = 5000
  for (let i = 0; i < total; i += batch) {
    let promises = []
    for (let j = i; j < i + batch; j++) {
      let msg = {
        chat: {
          id: 'stubChatId'
        },
        from: users[j % users.length]
      }
      promises.push(messageController.addMessage(msg))
    }
    await Promise.all(promises)

    if (i > milestone) {
      log.i(`added ${milestone} records`)
      milestone += 5000
    }
  }
}

try {
  populate().then(() => {
    log.i('done')
    process.exit()
  })
} catch (e) {
  log.e(e.stack)
  process.exit()
}
