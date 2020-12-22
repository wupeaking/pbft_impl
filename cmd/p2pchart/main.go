package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/wupeaking/pbft_impl/common/config"
	"github.com/wupeaking/pbft_impl/network"
	"github.com/wupeaking/pbft_impl/network/libp2p"
)

var port int64
var send bool

func init() {
	flag.Int64Var(&port, "port", 20200, "监听端口号")
	flag.BoolVar(&send, "send", true, "是否是发送")
	flag.Parse()
}

func main() {
	cfg := config.Configure{NetworkCfg: config.NetworkCfg{
		LocalAddr:  fmt.Sprintf("127.0.0.1:%d", port),
		PriVateKey: fmt.Sprintf("0xf25ccbf8a1bb36594d5f63e9564ca4c5d965ccf8b418e8717f2f68b600c%05d", port),
		LogLevel:   "debug",
	}}
	sw, err := libp2p.New(&cfg)
	if err != nil {
		panic(err)
	}
	sw.RegisterOnReceive("p2p", func(modelID string, msgBytes []byte, p *network.Peer) {
		var msgPage network.BroadcastMsg
		json.Unmarshal(msgBytes, &msgPage)
		logrus.Infof("peer: %v, recv msg: %v", p.ID, string(msgPage.Msg))
	})

	sw.Start()

	// sig := make(chan os.Signal, 0)

	rand.Seed(time.Now().UnixNano())
	for {
		time.Sleep(time.Duration(rand.Intn(6)) * time.Second)
		sw.Broadcast("p2p", &network.BroadcastMsg{
			ModelID: "p2p",
			Msg:     []byte(fmt.Sprintf("time: %v ", time.Now())),
		})
	}

	// if !send {
	// 	select {
	// 	case <-sig:
	// 		return
	// 	}
	// }
}
