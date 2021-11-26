const path = require('path')

module.exports = {
  target: 'node',
  mode: 'production',
  entry: './src/fastify.js',
  output: {
    filename: 'main.js',
    path: path.resolve(__dirname, 'dist'),
    libraryTarget: 'commonjs2'
  }
}
