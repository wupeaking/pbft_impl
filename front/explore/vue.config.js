
module.exports = {
    devServer: {
        proxy: { '/ws' : { target: 'http://localhost:8088', changeOrigin: true, ws: true, pathRewrite: { '^/ws': '/ws', } } }
    }
}