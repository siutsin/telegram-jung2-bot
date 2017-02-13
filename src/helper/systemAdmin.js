import _ from 'lodash'

export default class SystemAdmin {

  isAdmin (msg) {
    const adminList = process.env.ADMIN_ID.split(',')
    return (
    msg &&
    msg.from &&
    String(msg.from.id) &&
    _.includes(adminList, String(msg.from.id)))
  }

}
