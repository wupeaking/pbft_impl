package http_network

import (
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/gorilla/mux"
	"github.com/parnurzeal/gorequest"
	"github.com/wupeaking/pbft_impl/model"
	"github.com/wupeaking/pbft_impl/network"
)

type HTTPNetWork struct {
	Addrs        []string // 所有的节点地址
	LocalAddress string   // 本机地址
	msgQueue     chan interface{}
}

func New(nodeAddrs []string, local string) network.SwitcherI {
	return &HTTPNetWork{
		Addrs:        nodeAddrs,
		LocalAddress: local,
		msgQueue:     make(chan interface{}, 1000),
	}
}

func (hn *HTTPNetWork) Start() {
	r := mux.NewRouter()
	r.HandleFunc("/pbft_message", hn.consensusHandler).Methods("POST")
	r.HandleFunc("/block_meta", hn.blockMetaHandler).Methods("POST")
	r.HandleFunc("/transaction", hn.txHandler).Methods("POST")

	srv := &http.Server{
		Handler:      r,
		Addr:         hn.LocalAddress,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}
	go log.Fatal(srv.ListenAndServe())
}

func (hn *HTTPNetWork) Broadcast(msg interface{}) error {
	switch x := msg.(type) {
	case *model.PbftMessage:
		body, err := proto.Marshal(x)
		if err != nil {
			return err
		}
		request := gorequest.New()
		for _, addr := range hn.Addrs {
			request.Post(addr + "/pbft_message").Send(body).End()
		}
	case *model.BlockMeta:
		body, err := proto.Marshal(x)
		if err != nil {
			return err
		}
		request := gorequest.New()
		for _, addr := range hn.Addrs {
			request.Post(addr + "/block_meta").Send(body).End()
		}
	case *model.Tx:
		body, err := proto.Marshal(x)
		if err != nil {
			return err
		}
		request := gorequest.New()
		for _, addr := range hn.Addrs {
			request.Post(addr + "/transaction").Send(body).End()
		}
	}
	return nil
}

func (hn *HTTPNetWork) Recv() <-chan interface{} {
	return hn.msgQueue
}

func (hn *HTTPNetWork) consensusHandler(w http.ResponseWriter, r *http.Request) {
	content, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return
	}
	pbftMsg := model.PbftMessage{}
	if proto.Unmarshal(content, &pbftMsg) != nil {
		return
	}

	select {
	case hn.msgQueue <- &pbftMsg:
	default:
	}
	w.Write([]byte("ok"))
}

func (hn *HTTPNetWork) blockMetaHandler(w http.ResponseWriter, r *http.Request) {
	content, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return
	}
	meta := model.BlockMeta{}
	if proto.Unmarshal(content, &meta) != nil {
		return
	}

	select {
	case hn.msgQueue <- &meta:
	default:
	}
	w.Write([]byte("ok"))
}

func (hn *HTTPNetWork) txHandler(w http.ResponseWriter, r *http.Request) {
	content, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return
	}
	tx := model.Tx{}
	if proto.Unmarshal(content, &tx) != nil {
		return
	}

	select {
	case hn.msgQueue <- &tx:
	default:
	}
	w.Write([]byte("ok"))
}
