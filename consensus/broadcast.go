package consensus

import (
	"bytes"
	"time"

	"github.com/golang/protobuf/proto"
	cryptogo "github.com/wupeaking/pbft_impl/crypto"
	"github.com/wupeaking/pbft_impl/model"
	"github.com/wupeaking/pbft_impl/network"
	"github.com/wupeaking/pbft_impl/network/libp2p"
)

func (pbft *PBFT) LoadVerfierPeerIDs() error {
	for _, v := range pbft.verifiers {
		id, err := libp2p.PublicString2PeerID(cryptogo.Bytes2Hex(v.PublickKey))
		if err != nil {
			return err
		}
		pbft.verifierPeerID[id] = string(v.PublickKey)
	}
	return nil
}

func (pbft *PBFT) AddBroadcastTask(msg *model.PbftMessage) {
	select {
	case pbft.broadcastSig <- msg:
		return
	default:
	}
}

// 定时广播 由于网路原因 可能会导致一些节点不能一次成功收到消息 多次进行广播
func (pbft *PBFT) BroadcastMsgRoutine() {
	t := time.NewTicker(4 * time.Second)
	for {
		select {
		case <-t.C:
			// 定时广播
			if pbft.StopFlag {
				continue
			}
			if pbft.CurrentState() != model.States_ViewChanging {
				continue
			}

			if pbft.curBroadcastMsg == nil {
				continue
			}
			pbft.broadcastStateMsg(pbft.curBroadcastMsg)

		case msg := <-pbft.broadcastSig:
			//根据实际情况 判断是否需要广播
			// 1. 如果是第一次广播此消息 则全部广播
			if !pbft.compareMsg(msg, pbft.curBroadcastMsg) {
				// pbft.logger.Debugf("此次广播的消息和上次广播的消息不一致  msg: %#v, lastMsg: %#v", msg, pbft.curBroadcastMsg)
				pbft.broadcastStateMsg(pbft.curBroadcastMsg)
				pbft.curBroadcastMsg = msg
				continue
			}

			// 已经不是第一次广播此消息
			// 2. 当前类型消息 是否接收到其他节点发送过来 如果任意一个也没收到 则全网广播
			// 3. 如果收到某些验证节点发送的 则只向没有收到的验证节点广播
			// switch content := getPbftMsg(msg).(type) {
			// case *model.PbftGenericMessage:
			// 	msgInfo := content.GetInfo()
			// 	signers := pbft.sm.logMsg.FindMsg(msgInfo.GetSeqNum(), msgInfo.GetMsgType(), int(msgInfo.GetView()))
			// 	if len(signers) >= pbft.minNodeNum() {
			// 		pbft.curBroadcastMsg = msg
			// 		continue
			// 	}
			// 	peers, _ := pbft.switcher.Peers()
			// 	for _, p := range peers {
			// 		if sig, ok := pbft.verifierPeerID[p.ID]; ok {
			// 			// 说明这个节点是验证节点
			// 			// 如果接收到了这个节点发送的消息 那么认为这个节点也收到了本节点发送的消息 就不再往此节点发送消息
			// 			if _, ok := signers[sig]; ok {
			// 				continue
			// 			}
			// 		}
			// 		pbft.broadcastStateMsgToPeer(msg, p)
			// 	}
			// 	pbft.curBroadcastMsg = msg

			// case *model.PbftViewChange:
			// 	msgInfo := content.GetInfo()
			// 	signers := pbft.sm.logMsg.FindMsg(msgInfo.GetSeqNum(), msgInfo.GetMsgType(), int(msgInfo.GetView()))
			// 	if len(signers) >= pbft.minNodeNum() {
			// 		pbft.curBroadcastMsg = msg
			// 		continue
			// 	}
			// 	peers, _ := pbft.switcher.Peers()
			// 	for _, p := range peers {
			// 		if sig, ok := pbft.verifierPeerID[p.ID]; ok {
			// 			// 说明这个节点是验证节点
			// 			// 如果接收到了这个节点发送的消息 那么认为这个节点也收到了本节点发送的消息 就不再往此节点发送消息
			// 			if _, ok := signers[sig]; ok {
			// 				continue
			// 			}
			// 		}
			// 		pbft.broadcastStateMsgToPeer(msg, p)
			// 	}
			// 	pbft.curBroadcastMsg = msg
			// }
		}
	}
}

func (pbft *PBFT) broadcastStateMsg(msg *model.PbftMessage) error {
	body, err := proto.Marshal(msg)
	if err != nil {
		return err
	}
	msgPkg := network.BroadcastMsg{
		ModelID: "consensus",
		MsgType: model.BroadcastMsgType_send_pbft_msg,
		Msg:     body,
	}
	// pbft.logger.Debugf("向所有节点广播消息")
	return pbft.switcher.Broadcast("consensus", &msgPkg)
}

func (pbft *PBFT) broadcastStateMsgToPeer(msg *model.PbftMessage, peer *network.Peer) error {
	body, err := proto.Marshal(msg)
	if err != nil {
		return err
	}
	msgPkg := network.BroadcastMsg{
		ModelID: "consensus",
		MsgType: model.BroadcastMsgType_send_pbft_msg,
		Msg:     body,
	}
	// pbft.logger.Debugf("向%s广播消息", peer.ID)
	return pbft.switcher.BroadcastToPeer("consensus", &msgPkg, peer)
}

// compareMsg 比较两个消息是否是同一个消息 同时返回msgA的实际类型
func (pbft *PBFT) compareMsg(msgA *model.PbftMessage, msgB *model.PbftMessage) bool {
	mA := getPbftMsg(msgA)
	mB := getPbftMsg(msgB)
	switch ma := mA.(type) {
	case *model.PbftGenericMessage:
		mb, ok := mB.(*model.PbftGenericMessage)
		if !ok {
			// pbft.logger.Debugf("ma :%v mb: %v", ma, mb)
			return false
		}
		if ma.GetInfo().GetMsgType() == mb.GetInfo().GetMsgType() &&
			ma.GetInfo().GetSeqNum() == mb.GetInfo().GetSeqNum() &&
			ma.GetInfo().GetView() == mb.GetInfo().GetView() &&
			bytes.Compare(ma.GetInfo().GetSignerId(), mb.GetInfo().GetSignerId()) == 0 {
			return true
		}
		return false
	case *model.PbftViewChange:
		mb, ok := mB.(*model.PbftViewChange)
		if !ok {
			// pbft.logger.Debugf("ma :%v mb: %v", ma, mb)
			return false
		}
		if ma.GetInfo().GetMsgType() == mb.GetInfo().GetMsgType() &&
			ma.GetInfo().GetSeqNum() == mb.GetInfo().GetSeqNum() &&
			ma.GetInfo().GetView() == mb.GetInfo().GetView() &&
			bytes.Compare(ma.GetInfo().GetSignerId(), mb.GetInfo().GetSignerId()) == 0 {
			return true
		}
		// pbft.logger.Debugf("ma :%v mb: %v", ma, mb)
	}

	return false
}

func getPbftMsg(msg *model.PbftMessage) interface{} {
	if m := msg.GetGeneric(); m != nil {
		return m
	}
	if m := msg.GetViewChange(); m != nil {
		return m
	}
	return nil
}
