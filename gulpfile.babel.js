import gulp from 'gulp'
import mocha from 'gulp-mocha'
import runSequence from 'run-sequence'
import istanbul from 'gulp-babel-istanbul'
import standard from 'gulp-standard'
import injectModules from 'gulp-inject-modules'
import babel from 'gulp-babel'
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

// gulp.task('preIstanbul', () => gulp.src(jsFiles)
//   .pipe(istanbul())
//   .pipe(istanbul.hookRequire()))
//
// gulp.task('mochaAndIstanbul', () => gulp.src('./test/test.js', { read: false })
//   .pipe(mocha({
//     reporter: 'spec',
//     timeout: 60000,
//     compilers: 'js:babel-core/register'
//   }))
//   .pipe(istanbul.writeReports())
//   .pipe(istanbul.enforceThresholds({ thresholds: { global: 0 } }))
//   .on('error', () => process.exit(0)))

gulp.task('coverage', cb => {
  gulp.src(src)
    .pipe(istanbul())
    .pipe(istanbul.hookRequire()) // or you could use .pipe(injectModules())
    .on('finish', () => {
      gulp.src(testFiles)
        .pipe(babel())
        .pipe(injectModules())
        .pipe(mocha({
          reporter: 'spec',
          timeout: 60000,
          compilers: 'js:babel-core/register'
        }))
        .pipe(istanbul.writeReports())
        .pipe(istanbul.enforceThresholds({ thresholds: { global: 0 } }))
        .on('end', cb)
    })
})

gulp.task('default', callback => runSequence(
  'standard',
  'coverage',
  // 'preIstanbul',
  // 'mochaAndIstanbul',
  callback))
