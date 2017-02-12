import gulp from 'gulp'
import mocha from 'gulp-mocha'
import runSequence from 'run-sequence'
import istanbul from 'gulp-babel-istanbul'
import standard from 'gulp-standard'

const jsFiles = [
  '*.js',
  'src/**/*.js',
  'test/**/*.js'
]

gulp.on('stop', () => process.nextTick(() => process.exit(0)))

gulp.task('stress', () => gulp.src('./test/stress/testStress.js', {
  read: false
})
  .pipe(mocha({reporter: 'spec'}))
  .on('error', error => { throw error }))

gulp.task('standard', () => gulp.src(jsFiles)
  .pipe(standard())
  .pipe(standard.reporter('default', { breakOnError: true, quiet: false }))
)

gulp.task('preIstanbul', () => gulp.src(jsFiles)
  .pipe(istanbul())
  .pipe(istanbul.hookRequire()))

gulp.task('mochaAndIstanbul', () => gulp.src('./test/test.js', { read: false })
  .pipe(mocha({
    reporter: 'spec',
    timeout: 60000,
    compilers: 'js:babel-core/register'
  }))
  .pipe(istanbul.writeReports())
  .pipe(istanbul.enforceThresholds({ thresholds: { global: 0 } }))
  .on('error', () => process.exit(0)))

gulp.task('default', callback => runSequence(
  'standard',
  'preIstanbul',
  'mochaAndIstanbul',
  callback))
