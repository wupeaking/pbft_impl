package libp2p

import (
	"bufio"
	"encoding/json"

	pbftnet "github.com/wupeaking/pbft_impl/network"
)

func (p2p *P2PNetWork) dataStreamRecv(stream *P2PStream) {
	rw := bufio.NewReader(stream.stream)

outLoop:
	for {
		select {
		case <-stream.closeReadStrem:
			stream.stream.Conn().Close()
			p2p.Lock()
			delete(p2p.books, stream.stream.ID())
			p2p.Unlock()
			return
		default:
			msg, err := p2p.unpackageData(rw)
			if err != nil {
				logger.Infof("P2p Error reading from buffer")
				break outLoop
			}
			broadMsg, _ := json.Marshal(msg)
			onReceive := p2p.recvCB[msg.ModelID]
			if onReceive != nil {
				go onReceive(msg.ModelID, broadMsg, &pbftnet.Peer{ID: stream.stream.ID()})
			} else {
				logger.Debugf("当前消息ID没有相对应的处理模块 msgID: %s", msg.ModelID)
			}
		}
	}

	stream.stream.Conn().Close()
	p2p.Lock()
	delete(p2p.books, stream.stream.ID())
	p2p.Unlock()

	select {
	case stream.closeWriteStrem <- struct{}{}:
	default:
	}
}

func (p2p *P2PNetWork) dataStreamSend(stream *P2PStream) {
	rw := bufio.NewWriter(stream.stream)

outLoop:
	for {
		select {
		case <-stream.closeWriteStrem:
			stream.stream.Conn().Close()
			p2p.Lock()
			delete(p2p.books, stream.stream.ID())
			p2p.Unlock()
			return
		case msg := <-stream.broadcastMsgChan:
			msgBuf, err := p2p.packageData(msg)
			if err != nil {
				logger.Infof("P2p 广播消息编码失败, err: %v", err)
				break outLoop
			}
			_, err = rw.Write(msgBuf)
			if err != nil {
				logger.Debugf("P2p 广播消息失败, err: %v", err)
				break
			}
			err = rw.Flush()
			if err != nil {
				logger.Infof("P2p flush 广播消息失败, err: %v", err)
			}
		}
	}

	stream.stream.Conn().Close()
	p2p.Lock()
	delete(p2p.books, stream.stream.ID())
	p2p.Unlock()

	select {
	case stream.closeReadStrem <- struct{}{}:
	default:
	}
}
