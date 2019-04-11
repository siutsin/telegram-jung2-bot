import Jung2botUtil from './jung2botUtil'

const jung2botUtil = new Jung2botUtil()

export default class Help {
  async sendHelpMessage (message) {
    const helpMessage = `
冗員[jung2jyun4] Excess personnel in Cantonese

This bot is created for counting the number of message per participant in the group.

Rules:
1. Repeated messages will only be counted as ONE message.
2. Commands can only be used for ONCE per minute.

Commands:
/topten  show top ten 冗員s
/alljung  show all 冗員s
/junghelp  show help message

Issue/Suggestion: https://github.com/siutsin/telegram-jung2-bot/issues

May your 冗 power powerful -- Simon
`
    return jung2botUtil.sendMessage(message.chat.id, helpMessage)
  }
}
