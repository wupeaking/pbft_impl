package main

import (
	"flag"

	"github.com/wupeaking/pbft_impl/common/config"
	"github.com/wupeaking/pbft_impl/network/libp2p"
)

// 启动p2p的boostrap节点

var port string
var private string

//-private=0xa665c8da936eba27a48eae8c5f6d862e017c8b47715a11b0267570631af09d59 -port=10809
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
