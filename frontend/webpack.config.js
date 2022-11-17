const MiniCssExtractPlugin = require("mini-css-extract-plugin");
const devMode = process.env.NODE_ENV !== "production";

module.exports = {
    module: {
        rules: [
            {
                test: /\.(sa|sc|c)ss$/i,
                use: [
                    devMode ? "style-loader" : MiniCssExtractPlugin.loader,
                    "css-loader",
                    "postcss-loader",
                    "sass-loader",
                ],
            },
        ],
    },
    plugins: [].concat(devMode ? [] : [new MiniCssExtractPlugin()]),
};