package http_network

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/parnurzeal/gorequest"
	log "github.com/sirupsen/logrus"
	"github.com/wupeaking/pbft_impl/common/config"
	"github.com/wupeaking/pbft_impl/model"
	"github.com/wupeaking/pbft_impl/network"
)

var logger *log.Entry

func init() {
	logg := log.New()
	logg.SetLevel(log.DebugLevel)
	logg.SetReportCaller(true)
	logg.SetFormatter(&log.TextFormatter{
		DisableColors: true,
		FullTimestamp: true,
	})
	logger = logg.WithField("module", "P2P")
}

type HTTPNetWork struct {
	Addrs        []string // 所有的节点地址
	PeerIDs      []string //
	LocalAddress string   // 本机地址
	NodeID       string   // 节点ID
	msgQueue     chan *HTTPMsg
	peerBooks    *network.PeerBooks
	recvCB       map[string]network.OnReceive
	sync.RWMutex
}

type HTTPMsg struct {
	*network.BroadcastMsg
	*network.Peer
}

func New(nodeAddrs []config.NodeAddr, local string, nodeID string, cfg *config.Configure) network.SwitcherI {
	switch strings.ToLower(cfg.NetworkCfg.LogLevel) {
	case "debug":
		logger.Logger.SetLevel(log.DebugLevel)
	case "warn":
		logger.Logger.SetLevel(log.WarnLevel)
	case "info":
		logger.Logger.SetLevel(log.InfoLevel)
	case "error":
		logger.Logger.SetLevel(log.ErrorLevel)
	default:
		logger.Logger.SetLevel(log.InfoLevel)
	}
	addrs := make([]string, 0)
	peers := make([]string, 0)
	for i := range nodeAddrs {
		addrs = append(addrs, nodeAddrs[i].Address)
		peers = append(peers, nodeAddrs[i].PeerID)
	}
	return &HTTPNetWork{
		Addrs:        addrs,
		PeerIDs:      peers,
		LocalAddress: local,
		NodeID:       nodeID,
		msgQueue:     make(chan *HTTPMsg, 1000),
		peerBooks:    network.NewPeerBooks(),
		recvCB:       make(map[string]network.OnReceive),
	}
}

func (hn *HTTPNetWork) Start() error {
	r := mux.NewRouter()
	// r.HandleFunc("/pbft_message", hn.commonHander).Methods("POST")
	// r.HandleFunc("/block_meta", hn.commonHander).Methods("POST")
	// r.HandleFunc("/transaction", hn.commonHander).Methods("POST")
	// r.HandleFunc("/block/{num}", hn.commonHander).Methods("GET")
	// r.HandleFunc("/block", hn.commonHander).Methods("POST")
	r.HandleFunc("/broadcast", hn.commonHander).Methods("POST")

	srv := &http.Server{
		Handler:      r,
		Addr:         hn.LocalAddress,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}
	go func() {
		logger.Fatal(srv.ListenAndServe())
	}()

	go hn.Recv()

	return nil
}

func (hn *HTTPNetWork) Broadcast(modelID string, msg *network.BroadcastMsg) error {
	requestBody, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	// logger.Debugf("modeID: %s requestbody: %s", modelID, string(requestBody))
	switch msg.MsgType {
	case model.BroadcastMsgType_send_pbft_msg, model.BroadcastMsgType_send_block_meta,
		model.BroadcastMsgType_send_tx, model.BroadcastMsgType_request_load_block:
		for _, addr := range hn.Addrs {
			go func(addr string) {
				request := gorequest.New()
				logger.Debugf("向%s发起请求", addr)
				request.Post(addr + "/broadcast")
				// 必须在Method之后 添加头 这个包坑太多
				// Method之后 会清除SuperAgent
				request.Set("peer_id", hn.NodeID)
				request.Set("peer_address", "http://"+hn.LocalAddress)
				_, _, err := request.Send(string(requestBody)).End()
				if err != nil {
					logger.Debugf("P2P 广播出错, err: %v", err)
				}
			}(addr)
		}
	default:

	}
	return nil
}

func (hn *HTTPNetWork) BroadcastToPeer(modelID string, msg *network.BroadcastMsg, p *network.Peer) error {
	requestBody, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	switch msg.MsgType {
	case model.BroadcastMsgType_send_pbft_msg, model.BroadcastMsgType_send_block_meta,
		model.BroadcastMsgType_send_tx, model.BroadcastMsgType_request_load_block,
		model.BroadcastMsgType_send_specific_block:
		go func() {
			request := gorequest.New()
			logger.Debugf("向%s发起请求", p.Address)
			request.Post(p.Address + "/broadcast")
			// 必须在Method之后 添加头 这个包坑太多
			// Method之后 会清除SuperAgent
			request.Set("peer_id", hn.NodeID)
			request.Set("peer_address", "http://"+hn.LocalAddress)
			_, _, err := request.Send(string(requestBody)).End()
			if err != nil {
				logger.Debugf("P2P 广播出错, err: %v", err)
			}
		}()
	default:

	}
	return nil
}

func (hn *HTTPNetWork) BroadcastExceptPeer(modelID string, msg *network.BroadcastMsg, p *network.Peer) error {
	requestBody, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	switch msg.MsgType {
	case model.BroadcastMsgType_send_pbft_msg, model.BroadcastMsgType_send_block_meta,
		model.BroadcastMsgType_send_tx, model.BroadcastMsgType_request_load_block,
		model.BroadcastMsgType_send_specific_block:
		for _, addr := range hn.Addrs {
			if addr == p.Address {
				continue
			}
			go func(addr string) {
				request := gorequest.New()
				logger.Debugf("向%s发起请求", addr)
				request.Post(addr + "/broadcast")
				// 必须在Method之后 添加头 这个包坑太多
				// Method之后 会清除SuperAgent
				request.Set("peer_id", hn.NodeID)
				request.Set("peer_address", "http://"+hn.LocalAddress)
				_, _, err := request.Send(string(requestBody)).End()
				if err != nil {
					logger.Debugf("P2P 广播出错, err: %v", err)
				}
			}(addr)
		}
	default:
	}
	return nil
}

func (hn *HTTPNetWork) RemovePeer(p *network.Peer) error {
	return nil
}

func (hn *HTTPNetWork) RegisterOnReceive(modelID string, callBack network.OnReceive) error {
	hn.Lock()
	hn.recvCB[modelID] = callBack
	hn.Unlock()
	return nil
}

func (hn *HTTPNetWork) Peers() ([]*network.Peer, error) {
	peers := make([]*network.Peer, 0)
	for i := range hn.Addrs {
		peers = append(peers, &network.Peer{
			ID:      hn.PeerIDs[i],
			Address: hn.Addrs[i],
		})

	}
	return peers, nil
}

func (hn *HTTPNetWork) Recv() {
	logger.Debugf("开始接收消息")
	for {
		select {
		case msg := <-hn.msgQueue:
			broadMsg, err := json.Marshal(msg.BroadcastMsg)
			if err != nil {
				logger.Debugf("解码接收到的关播内容出错 err: %s", err.Error())
				continue
			}
			onReceive := hn.recvCB[msg.ModelID]
			if onReceive != nil {
				go onReceive(msg.ModelID, broadMsg, msg.Peer)
			} else {
				logger.Debugf("当前消息ID没有相对应的处理模块 msgID: %s", msg.ModelID)
			}
		}
	}
}
