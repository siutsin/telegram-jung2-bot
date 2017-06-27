import gulp from 'gulp'
import mocha from 'gulp-mocha'
import runSequence from 'run-sequence'
import standard from 'gulp-standard'
// import log from 'log-to-file-and-console-node'

const src = 'src/**/*.js'
const testFiles = 'test/**/*.js'
const jsFiles = [
  '*.js',
  src,
  testFiles
]

gulp.on('stop', () => process.nextTick(() => process.exit(0)))

gulp.task('stress', () => gulp.src('./test/stress/testStress.js', {read: false})
  .pipe(mocha({
    reporter: 'spec',
    timeout: 60000,
    compilers: 'js:babel-core/register'
  }))
  .on('error', (e) => {
    console.log(e.message)
    process.exit(0)
  }))

gulp.task('standard', () => gulp.src(jsFiles)
  .pipe(standard())
  .pipe(standard.reporter('default', { breakOnError: true, quiet: false }))
)

gulp.task('default', callback => runSequence(
  'standard',
  callback))
