const WEEKDAY_OPTIONS = {
  SUN: 1,
  MON: 2,
  TUE: 4,
  WED: 8,
  THU: 16,
  FRI: 32,
  SAT: 64
}

class WorkdayHelper {
  workdayStringToBinary (workday) {
    return workday.split(',').map(o => WEEKDAY_OPTIONS[o]).reduce((acc, cur) => acc + cur, 0)
  }

  isWeekdayMatchBinary (workday, binary) {
    return !!(WEEKDAY_OPTIONS[workday] & binary)
  }
}

module.exports = WorkdayHelper
