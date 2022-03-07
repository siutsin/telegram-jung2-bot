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
/topTen  show top ten 冗員s
/topDiver  show top ten 潛水員s (潛得太深會搵唔到)
/allJung  show all 冗員s
/jungHelp  show help message

Admin Only:
/enableAllJung  enable /alljung command
/disableAllJung  disable /alljung command
/setOffFromWorkTimeUTC set offFromWork time in UTC

Issue/Suggestion: https://github.com/siutsin/telegram-jung2-bot/issues

May your 冗 power powerful
`
    await this.telegram.sendMessage(chatId, helpMessage, { disable_web_page_preview: true })
    return helpMessage
  }
}

module.exports = Help
