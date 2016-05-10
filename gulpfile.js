'use strict';

var gulp = require('gulp');
var jshint = require('gulp-jshint');
var stylish = require('jshint-stylish');
var mocha = require('gulp-mocha');
var runSequence = require('run-sequence');
var istanbul = require('gulp-istanbul');
var _ = require('lodash');

var jsFiles = [
  '*.js',
  './model/**.js',
  './route/**.js',
  './controller/**.js',
  './ds/**.js'
];
var checkForJSHint = _.union(jsFiles, [
  './test/**.js'
]);

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
    .pipe(istanbul())
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
    .pipe(istanbul.enforceThresholds({thresholds: {global: 100}}))
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

gulp.task('stress', function () {
  return gulp.src('./test/stress/testStress.js', {
      read: false
    })
    .pipe(mocha({
      reporter: 'spec'
    }))
    .on('error', function (error) {
      throw error;
    });
});
