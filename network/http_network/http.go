package http_network

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/parnurzeal/gorequest"
	log "github.com/sirupsen/logrus"
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
	LocalAddress string   // 本机地址
	NodeID       string   // 节点ID
	msgQueue     chan *HTTPMsg
	peerBooks    *network.PeerBooks
	recvCB       map[string]network.OnReceive
}

type HTTPMsg struct {
	*network.BroadcastMsg
	*network.Peer
}

func New(nodeAddrs []string, local string, nodeID string) network.SwitcherI {
	return &HTTPNetWork{
		Addrs:        nodeAddrs,
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
	go logger.Fatal(srv.ListenAndServe())

	go hn.Recv()

	return nil
}

func (hn *HTTPNetWork) Broadcast(modelID string, msg *network.BroadcastMsg) error {
	requestBody, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	request := gorequest.New()
	request.Header.Add("peer_id", hn.NodeID)
	request.Header.Add("peer_address", "http://"+hn.LocalAddress)

	switch msg.MsgType {
	case model.BroadcastMsgType_send_pbft_msg, model.BroadcastMsgType_send_block_meta,
		model.BroadcastMsgType_send_tx, model.BroadcastMsgType_request_load_block:
		go func() {
			for _, addr := range hn.Addrs {
				request.Post(addr + "/broadcast").Send(requestBody).End()
			}
		}()
	default:

	}
	return nil
}

func (hn *HTTPNetWork) BroadcastToPeer(modelID string, msg *network.BroadcastMsg, p *network.Peer) error {
	requestBody, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	request := gorequest.New()
	request.Header.Add("peer_id", hn.NodeID)
	request.Header.Add("peer_address", "http://"+hn.LocalAddress)

	switch msg.MsgType {
	case model.BroadcastMsgType_send_pbft_msg, model.BroadcastMsgType_send_block_meta,
		model.BroadcastMsgType_send_tx, model.BroadcastMsgType_request_load_block:
		go func() {
			request.Post(p.Address + "/broadcast").Send(requestBody).End()
		}()
	default:

	}
	return nil
}

func (hn *HTTPNetWork) RemovePeer(p *network.Peer) error {
	return nil
}

func (hn *HTTPNetWork) RegisterOnReceive(modelID string, callBack network.OnReceive) error {
	hn.recvCB[modelID] = callBack
	return nil
}

func (hn *HTTPNetWork) Recv() {
	for {
		select {
		case msg := <-hn.msgQueue:
			broadMsg, _ := json.Marshal(msg.BroadcastMsg)
			onReceive := hn.recvCB[msg.ModelID]
			if onReceive != nil {
				go onReceive(msg.ModelID, broadMsg, msg.Peer)
			}
		}
	}
}
