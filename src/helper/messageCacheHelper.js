'use strict'

function bsearchMin (a, b, test) {
  // b is true and valid
  while (b - a > 1) {
    var mid = Math.floor((a + b) / 2)
    test(mid) ? b = mid : a = mid
  }
  return b
}

function bsearchMax (a, b, test) {
  // a is true and valid
  while (b - a > 1) {
    var mid = Math.floor((a + b) / 2)
    test(mid) ? a = mid : b = mid
  }
  return a
}

class User {
  constructor (userId, details) {
    this.id = userId
    this.timestamp = []
    this.details = details
  }

  lastTimestamp () {
    let l = this.timestamp.length
    if (l === 0) return null
    return this.timestamp[l - 1]
  }

  addTimestamp (t) {
    // t is unix time accurate to second
    this.timestamp.push(t)
  }

  numMsgBetween (start, end) {
    let ts = this.timestamp
    let len = ts.length
    if (len === 0 || ts[len - 1] < start || end < ts[0]) return 0

    let mi = bsearchMin(-1, len - 1, function (i) {
      return start <= ts[i]
    })
    let mx = bsearchMax(0, len, function (i) {
      return ts[i] <= end
    })
    return mx - mi + 1
  }

  name () {
    let d = this.details
    if (d.first_name && d.last_name) return d.first_name + ' ' + d.last_name
    if (d.first_name) return d.first_name
    if (d.last_name) return d.last_name
    if (d.username) return d.username
    return ''
  }

  sort () {
    this.timestamp.sort((a, b) => a - b)
  }

  clearTimestampBefore (time) {
    let ts = this.timestamp
    let len = ts.length
    if (len === 0 || time < ts[0]) return

    let ix = bsearchMax(0, len, function (i) {
      return ts[i] < len
    })

    // todo: not sure whether V8 would actually free memory
    this.timestamp.splice(0, ix + 1)
  }
}

class Group {
  constructor (id, details) {
    this.id = id
    this.users = new Map()
    this.details = details
  }

  hasUser (userId) {
    return this.users.has(userId)
  }

  getUser (userId) {
    return this.users.get(userId)
  }

  setUser (userId, details) {
    this.users.set(userId, new User(userId, details))
  }

  replaceUserDetails (uid, details) {
    var u = this.getUser(uid)
    if (typeof u === 'undefined') return
    u.details = details
  }

  patchUserDetails (userId, details) {
    var u = this.getUser(userId)
    for (let k in details) {
      if (details.hasOwnProperty(k)) u.details[k] = details[k]
    }
  }

  rank (startTime, endTime) {
    let rank = []
    for (let u of this.users.values()) {
      rank.push({
        user: u.details,
        numMsg: u.numMsgBetween(startTime, endTime),
        lastTimestamp: u.lastTimestamp()
      })
    }
    rank.sort(function (a, b) {
      var t = b.numMsg - a.numMsg
      if (t !== 0) return t
      return b.lastTimestamp - a.lastTimestamp     // latest message first
    })

    // total msg
    let total = 0
    for (let i = 0; i < rank.length; i++) total += rank[i].numMsg
    return {
      total: total,
      rank: rank
    }
  }

  sort () {
    for (let u of this.users.values()) u.sort()
  }

  clearTimestampBefore (time) {
    for (let u of this.users.values()) u.clearTimestampBefore(time)
  }
}

class MessageCache {
  constructor () {
    this.groups = new Map()
  }

  static isValid (msg) {
    var isDefined = x => typeof x !== 'undefined'
    return isDefined(msg) && isDefined(msg.chat) && isDefined(msg.from) && isDefined(msg.date)
  }

  /**
   *
   * Assumption:
   *  1. late coming msg always has larger or equal msg.date
   *
   * @param msg
   */
  addMessage (msg) {
    if (!MessageCache.isValid(msg)) return false

    let gid = msg.chat.id
    let uid = msg.from.id

    // add group if not exist
    if (!this.hasGroup(gid)) this.setGroup(gid, msg.chat)
    this.replaceGroupDetails(gid, msg.chat)
    let g = this.getGroup(gid)

    // add user if not exist
    if (!g.hasUser(uid)) g.setUser(uid, msg.from)
    g.replaceUserDetails(uid, msg.from)
    let u = g.getUser(uid)

    u.addTimestamp(msg.date)

    return true
  }

  hasGroup (gid) {
    return this.groups.has(gid)
  }

  getGroup (gid) {
    return this.groups.get(gid)
  }

  setGroup (gid, details) {
    return this.groups.set(gid, new Group(gid, details))
  }

  replaceGroupDetails (gid, details) {
    var g = this.getGroup(gid)
    if (typeof g === 'undefined') return
    g.details = details
  }

  patchGroupDetails (gid, details) {
    var g = this.getGroup(gid)
    if (typeof g === 'undefined') return
    for (let k in details) {
      if (details.hasOwnProperty(k)) g.details[k] = details[k]
    }
  }

  /**
   *
   * return rank considert number of message between startTime and endTime inclusively
   *
   * @param gid
   * @param startTime - unix timestamp in second
   * @param endTime - unix timestamp in second
   * @returns {*}
   */
  rankByGroupTimestamp (gid, startTime, endTime) {
    let g = this.getGroup(gid)

    if (typeof startTime !== 'number' || isNaN(startTime)) {
      throw new Error('start time must be a number ' + startTime)
    }
    if (typeof endTime !== 'number' || isNaN(endTime)) {
      throw new Error('end time must be a number ' + endTime)
    }

    if (typeof g === 'undefined') return []
    let gr = g.rank(startTime, endTime)
    return {
      group: g.details,
      total: gr.total,
      rank: gr.rank
    }
  }

  rankByGroupDate (gid, startDate, endDate) {
    let unixTime = (d) => Math.round(d.getTime() / 1000)
    return this.rankByGroup(gid, unixTime(startDate), unixTime(endDate))
  }

  /**
   *
   * re-sort just in case it violate assumption 1
   *
   */
  sort () {
    for (let g of this.groups.values()) g.sort()
  }

  /**
   *
   * free unnecessary timestamp
   *
   * @param time
   */
  clearTimestampBefore (time) {
    for (let g of this.groups.values()) g.clearTimestampBefore(time)
  }
}

module.exports = MessageCache

// var stubMsg = {
//   chat: {
//     id: 123,
//     type: 'group',    //  “private”, “group”, “supergroup” or “channel”
//     title: '',        // optional
//     username: '',     // Optional. Username, for private chats and channels if available
//     first_name: '',   // Optional. First name of the other party in a private chat
//     last_name: ''     // Optional. Last name of the other party in a private chat
//   },
//   from: {
//     id: 123,             // integer
//     username: 'stubUsername',
//     first_name: 'stubFirstName',  // optional
//     last_name: 'stubLastName'     // optional
//   },
//   date: 1462008157,
//   text: 'hi'
// }
