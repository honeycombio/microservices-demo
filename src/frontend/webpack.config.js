const path = require('path');

module.exports = {
  entry: './static/js/instrumentation-load.js',
  output: {
    path: path.resolve(__dirname, 'dist'),
    filename: 'instrumentation-load.js',
  },
  mode: 'development',
  devtool: 'source-map',
  resolve: {
    modules: [
        path.resolve(__dirname, 'node_modules'),
        'node_modules'
    ]
  }
};