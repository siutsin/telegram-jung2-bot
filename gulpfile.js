var gulp = require('gulp');
var jshint = require('gulp-jshint');
var stylish = require('jshint-stylish');
var mocha = require('gulp-mocha');
var runSequence = require('run-sequence');
var istanbul = require('gulp-istanbul');
var _ = require('lodash');

var jsFiles = [
  '*.js',
  './database/**.js'
];
var checkForJSHint = _.union(jsFiles, ['./test/**.js']);

gulp.on('stop', function () {
  process.nextTick(function () {
    process.exit(0);
  });
});

gulp.task('jshint', function () {
  return gulp.src(checkForJSHint)
    .pipe(jshint())
    .pipe(jshint.reporter(stylish));
});

gulp.task('preIstanbul', function () {
  return gulp.src(jsFiles)
    // Covering files
    .pipe(istanbul())
    // Force `require` to return covered files
    .pipe(istanbul.hookRequire());
});

gulp.task('mochaAndIstanbul', function () {
  return gulp.src('./test/startTest.js', {
      read: false
    })
    .pipe(mocha({
      reporter: 'spec'
    }))
    .pipe(istanbul.writeReports())
    // Enforce a coverage of 100%
    .pipe(istanbul.enforceThresholds({thresholds: {global: 0}}))
    .on('error', function (error) {
      throw error;
    });
});

gulp.task('default', function (callback) {
  runSequence(
    'jshint',
    'preIstanbul',
    'mochaAndIstanbul',
    callback);
});
