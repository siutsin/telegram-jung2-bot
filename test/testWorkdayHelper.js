const test = require('ava')
const WorkdayHelper = require('../src/workdayHelper')

test('workdayStringToBinary', async t => {
  const workdayHelper = new WorkdayHelper()
  const b1 = workdayHelper.workdayStringToBinary('MON,TUE,WED,THU,FRI')
  t.is(b1, 62)
  const b2 = workdayHelper.workdayStringToBinary('MON,TUE,WED,THU')
  t.is(b2, 30)
  const b3 = workdayHelper.workdayStringToBinary('SUN,MON,TUE,WED,THU')
  t.is(b3, 31)
})

test('isWeekdayMatchBinary - MON to FRI - true', async t => {
  const workdayHelper = new WorkdayHelper()
  const monToFri = 62
  for (const day of ['MON', 'TUE', 'WED', 'THU', 'FRI']) {
    t.true(workdayHelper.isWeekdayMatchBinary(day, monToFri))
  }
})

test('isWeekdayMatchBinary - MON to FRI - false', async t => {
  const workdayHelper = new WorkdayHelper()
  const monToFri = 62
  for (const day of ['SAT', 'SUN']) {
    t.false(workdayHelper.isWeekdayMatchBinary(day, monToFri))
  }
})

test('isWeekdayMatchBinary - SUN to THU - true', async t => {
  const workdayHelper = new WorkdayHelper()
  const monToFri = 31
  for (const day of ['SUN', 'MON', 'TUE', 'WED', 'THU']) {
    t.true(workdayHelper.isWeekdayMatchBinary(day, monToFri))
  }
})

test('isWeekdayMatchBinary - SUN to THU - false', async t => {
  const workdayHelper = new WorkdayHelper()
  const monToFri = 31
  for (const day of ['FRI', 'SAT']) {
    t.false(workdayHelper.isWeekdayMatchBinary(day, monToFri))
  }
})
