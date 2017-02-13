export default class Root {

  static root () {
    return (req, res) => {
      res.json({
        status: 'OK',
        desc: 'For UpTimeRobot'
      })
    }
  }

}
