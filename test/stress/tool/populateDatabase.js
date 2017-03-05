'use strict'

const MessageController = require('../.././mongoMessage')
const mongoose = require('mongoose')
const Message = require('../.././message')
const co = require('co')
const faker = require('faker')

// connect to local database for testing
var connectionString = '127.0.0.1:27017/jung2botTest'
mongoose.connect(connectionString, {db: {nativeParser: true}})

// wrap MessageController.addMessage to promise
function addMessage (msg) {
  return new Promise(function (resolve, reject) {
    MessageController.addMessage(msg, resolve)
  })
}

let nUsers = 20
let users = []
for (let i = 0; i < nUsers; i++) {
  users.push({
    id: `id-${i}`,
    username: faker.internet.userName(),
    first_name: faker.name.firstName(),
    last_name: faker.name.lastName()
  })
}
// make some repetition
for (let i = 0; i < nUsers; i++) {
  for (let j = 0; j < i; j++) {
    users.push(users[i])
  }
}

co(function * () {
  // remove all data
  yield Message.remove({})

  // insert data
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
      promises.push(addMessage(msg))
    }
    yield Promise.all(promises)

    if (i > milestone) {
      console.log('added', milestone, 'records')
      milestone += 5000
    }
  }
}).then(function () {
  console.log('done')
  process.exit()
}).catch(function (e) {
  console.error(e.stack)
  process.exit()
})
