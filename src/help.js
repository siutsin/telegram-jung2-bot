const Telegram = require('./telegram')

class Help {
  constructor () {
    this.telegram = new Telegram()
  }

  async sendHelpMessage ({ chatId, chatTitle }) {
    const helpMessage = `
圍爐區: ${chatTitle}

冗員[jung2jyun4] Excess personnel in Cantonese

This bot is created for counting the number of message per participant in the group.

Commands:
/topten  show top ten 冗員s
/topdiver  show top ten 潛水員s (潛得太深會搵唔到)
/alljung  show all 冗員s
/junghelp  show help message

Admin Only:
/enablealljung  enable /alljung command
/disablealljung  disable /alljung command

Issue/Suggestion: https://github.com/siutsin/telegram-jung2-bot/issues

May your 冗 power powerful -- Simon
`
    await this.telegram.sendMessage(chatId, helpMessage)
    return helpMessage
  }
}

module.exports = Help
