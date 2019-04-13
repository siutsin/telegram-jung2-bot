import test from 'ava'
import path from 'path'
import OffFromWork from '../src/offFromWork'
import dotenv from 'dotenv'

dotenv.config({ path: path.resolve(__dirname, '.env.testing') })

const offFromWork = new OffFromWork()

test.failing('off', async t => {
  const response = await offFromWork.off()
  t.is(response, 'dummy')
})
