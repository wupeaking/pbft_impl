package http_network

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/parnurzeal/gorequest"
	"github.com/wupeaking/pbft_impl/model"
	"github.com/wupeaking/pbft_impl/network"
)

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
	r.HandleFunc("/pbft_message", hn.commonHander).Methods("POST")
	r.HandleFunc("/block_meta", hn.commonHander).Methods("POST")
	r.HandleFunc("/transaction", hn.commonHander).Methods("POST")
	r.HandleFunc("/block_header/{num}", hn.commonHander).Methods("GET")

	srv := &http.Server{
		Handler:      r,
		Addr:         hn.LocalAddress,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}
	go log.Fatal(srv.ListenAndServe())

	go hn.Recv()

	return nil
}

func (hn *HTTPNetWork) Broadcast(modelID string, msg *network.BroadcastMsg) error {
	switch msg.MsgType {
	case model.BroadcastMsgType_send_pbft_msg:
		body, err := json.Marshal(msg)
		if err != nil {
			return err
		}
		go func() {
			request := gorequest.New()
			request.Header.Add("peer_id", hn.NodeID)
			request.Header.Add("peer_address", hn.LocalAddress)
			for _, addr := range hn.Addrs {
				request.Post(addr + "/pbft_message").Send(body).End()
			}
		}()

	case model.BroadcastMsgType_send_block_meta:
		body, err := json.Marshal(msg)
		if err != nil {
			return err
		}
		go func() {
			request := gorequest.New()
			request.Header.Add("peer_id", hn.NodeID)
			request.Header.Add("peer_address", hn.LocalAddress)
			for _, addr := range hn.Addrs {
				request.Post(addr + "/block_meta").Send(body).End()
			}
		}()

	case model.BroadcastMsgType_send_tx:
		body, err := json.Marshal(msg)
		if err != nil {
			return err
		}
		go func() {
			request := gorequest.New()
			request.Header.Add("peer_id", hn.NodeID)
			request.Header.Add("peer_address", hn.LocalAddress)
			for _, addr := range hn.Addrs {
				request.Post(addr + "/transaction").Send(body).End()
			}
		}()
	}
	return nil
}

func (hn *HTTPNetWork) BroadcastToPeer(msg *network.BroadcastMsg, p *network.Peer) error {
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
