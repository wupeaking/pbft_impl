package main

import (
	"flag"

	"github.com/wupeaking/pbft_impl/common/config"
	"github.com/wupeaking/pbft_impl/network/libp2p"
)

// 启动p2p的boostrap节点

var port string
var private string

//-private=0xbcf0d9e24b6b12f0d401eb1d133ca104001b64f17a6e8d629a07e4b90aa4e10e -port=10809
func init() {
	flag.StringVar(&port, "port", "805", "监听的端口号")
	flag.StringVar(&private, "private", "", "私钥")
}

func main() {
	flag.Parse()

	cfg := &config.Configure{NetworkCfg: config.NetworkCfg{
		NetMode:    "p2p",
		LocalAddr:  "0.0.0.0:" + port,
		PriVateKey: private,
		LogLevel:   "info",
		Bootstrap:  true,
	}}
	switcher, err := libp2p.New(cfg)
	if err != nil {
		panic(err)
	}
	err = switcher.Start()
	if err != nil {
		panic(err)
	}
	select {}
}
