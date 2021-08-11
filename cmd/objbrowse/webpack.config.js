const path = require('path');
const HtmlWebpackPlugin = require('html-webpack-plugin');

module.exports = (env, argv) => {
  const outDir = argv.mode === 'development' ? 'web/dist-dev' : 'web/dist-prod';

  return {
    entry: './web/index.tsx',
    resolve: {
      extensions: ['.js', '.jsx', '.ts', '.tsx'],
    },
    output: {
      filename: 'main.js',
      path: path.resolve(__dirname, outDir),
      clean: true,
    },
    cache: {
      type: 'filesystem',
    },
    module: {
      rules: [
        {
          test: /\.tsx?$/,
          use: 'ts-loader',
          exclude: /node_modules/,
        },
        {
          test: /\.css$/,
          use: [
            'style-loader',
            'css-loader'
          ],
        }],
    },
    plugins: [
      new HtmlWebpackPlugin({
        template: "./web/index.ejs",
      }),
    ],
  };
};
