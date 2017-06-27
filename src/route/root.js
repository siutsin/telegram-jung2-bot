export default class Root {
  root () {
    return (req, res) => {
      res.json({
        status: 'OK',
        desc: 'For UpTimeRobot'
      })
    }
  }
}
