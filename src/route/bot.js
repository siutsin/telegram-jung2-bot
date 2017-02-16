export default class Bot {

  constructor (bot) {
    this.bot = bot
  }

  root () {
    return (req, res) => {
      this.bot.processUpdate(req.body)
      res.sendStatus(200)
    }
  }

}
