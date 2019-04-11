export default function factory () {
  return {
    require: ['esm'],
    files: [
      'test/*.js'
    ],
    sources: [
      'src/**/*.js'
    ],
    cache: true,
    concurrency: 5,
    failFast: true
  }
}
