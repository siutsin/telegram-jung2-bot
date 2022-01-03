module.exports = () => {
  return {
    files: [
      'test/*.js'
    ],
    cache: true,
    concurrency: 5,
    failFast: false,
    timeout: '30s'
  }
}
