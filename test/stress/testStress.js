import '../../src/env'
import MessageController from '../../src/controller/message'

const messageController = new MessageController()

const sample = {
  chat: {
    id: 'stubChatId'
  }
}

const repeat = async (n, recreatePromise) => {
  let promises = []
  for (let i = 0; i < n; i++) promises.push(recreatePromise)
  return await Promise.all(promises)
}

describe('MessageStressTest', () => {
  describe('getTopTen', function () {
    it('test 1: once', async () => {
      try {
        await messageController.getTopTen(sample, true)
      } catch (e) {
        console.error(e.stack)
      }
    })

    it('test 2: repeat 10 times', async () => {
      try {
        await repeat(10, messageController.getTopTen(sample, true))
      } catch (e) {
        console.error(e.stack)
      }
    })
  })

  describe('getAllJung', () => {
    it('test 1: once', async () => {
      try {
        await messageController.getAllJung(sample, true)
      } catch (e) {
        console.error(e.stack)
      }
    })

    it('test 2: repeat 10 times', async () => {
      try {
        await repeat(10, messageController.getAllJung(sample, true))
      } catch (e) {
        console.error(e.stack)
      }
    })
  })

  describe('simulating real usage', () => {
    it('repeat 400 times', async () => {
      try {
        await repeat(400, messageController.getAllJung(sample, true))
      } catch (e) {
        console.error(e.stack)
      }
    })
  })
})
