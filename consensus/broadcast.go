package consensus

import (
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/wupeaking/pbft_impl/model"
	"github.com/wupeaking/pbft_impl/network"
)

func (pbft *PBFT) AddBroadcastTask(msg *model.PbftMessage) {
	select {
	case pbft.broadcastSig <- msg:
		return
	default:
	}
}

// 定时广播 由于网路原因 可能会导致一些节点不能一次成功收到消息 多次进行广播
func (pbft *PBFT) BroadcastMsgRoutine() {
	t := time.NewTicker(2 * time.Second)
	for {
		select {
		case <-t.C:
			// 定时广播
			if pbft.StopFlag {
				continue
			}
			if pbft.CurrentState() == model.States_NotStartd {
				continue
			}
			if pbft.curBroadcastMsg == nil {
				continue
			}
			if err := pbft.broadcastStateMsg(pbft.curBroadcastMsg); err != nil {
				pbft.logger.Debugf("定时广播消息出错 err: %v", err)
			}

		case msg := <-pbft.broadcastSig:
			pbft.curBroadcastMsg = msg
			pbft.broadcastStateMsg(pbft.curBroadcastMsg)
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
	return pbft.switcher.Broadcast("consensus", &msgPkg)
}
