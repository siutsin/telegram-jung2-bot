var _ = require('lodash')

var jungBotSystemAdminHelper = {}

jungBotSystemAdminHelper.isAdmin = function (msg) {
  var adminList = process.env.ADMIN_ID.split(',')
  var isAdmin = (msg && msg.from && String(msg.from.id) && _.includes(adminList, String(msg.from.id)))
  return isAdmin
}

module.exports = jungBotSystemAdminHelper
