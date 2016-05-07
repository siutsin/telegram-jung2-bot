'use strict';

var express = require('express');
var router = express.Router();

router.get('/', function(req, res) {
  res.json({
    status: 'OK',
    desc: 'For UpTimeRobot'
  });
});

module.exports = router;
