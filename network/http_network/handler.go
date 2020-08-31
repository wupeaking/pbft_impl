package http_network

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/wupeaking/pbft_impl/network"
)

func (hn *HTTPNetWork) consensusHandler(w http.ResponseWriter, r *http.Request) {
	content, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return
	}
	var revMsg network.BroadcastMsg
	if json.Unmarshal(content, &revMsg) != nil {
		return
	}
	peer := network.Peer{
		ID:      r.Header.Get("peer_id"),
		Address: r.Header.Get("peer_address"),
	}

	select {
	case hn.msgQueue <- &HTTPMsg{
		&revMsg,
		&peer,
	}:
	default:
	}
	w.Write([]byte("ok"))
}

func (hn *HTTPNetWork) blockMetaHandler(w http.ResponseWriter, r *http.Request) {
	content, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return
	}
	var revMsg network.BroadcastMsg
	if json.Unmarshal(content, &revMsg) != nil {
		return
	}
	peer := network.Peer{
		ID:      r.Header.Get("peer_id"),
		Address: r.Header.Get("peer_address"),
	}

	select {
	case hn.msgQueue <- &HTTPMsg{
		&revMsg,
		&peer,
	}:
	default:
	}
	w.Write([]byte("ok"))
}

func (hn *HTTPNetWork) txHandler(w http.ResponseWriter, r *http.Request) {
	content, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return
	}
	var revMsg network.BroadcastMsg
	if json.Unmarshal(content, &revMsg) != nil {
		return
	}
	peer := network.Peer{
		ID:      r.Header.Get("peer_id"),
		Address: r.Header.Get("peer_address"),
	}
	select {
	case hn.msgQueue <- &HTTPMsg{
		&revMsg,
		&peer,
	}:
	default:
	}
	w.Write([]byte("ok"))
}

func (hn *HTTPNetWork) commonHander(w http.ResponseWriter, r *http.Request) {
	content, err := ioutil.ReadAll(r.Body)
	// logger.Debugf("收到请求 url: %s, content: %s", r.RequestURI, string(content))
	if err != nil {
		logger.Debugf("读取请求内容出错 %s", err.Error())
		return
	}
	var revMsg network.BroadcastMsg
	if err := json.Unmarshal(content, &revMsg); err != nil {
		logger.Debugf("解码请求内容出错 %s", err.Error())
		return
	}
	peer := network.Peer{
		ID:      r.Header.Get("peer_id"),
		Address: r.Header.Get("peer_address"),
	}

	select {
	case hn.msgQueue <- &HTTPMsg{
		&revMsg,
		&peer,
	}:
	default:
	}
	w.Write([]byte("ok"))
}
