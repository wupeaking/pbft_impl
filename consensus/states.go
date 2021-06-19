package consensus

import (
	"bytes"
	"sync"
	"time"

	"github.com/wupeaking/pbft_impl/model"
)

type StateMachine struct {
	state model.States
	sync.Mutex
	// // 区块编号  <---->  接收到的消息日志 (消息类型-视图编号: 消息)
	// logMsg   LogMsgCollection
	// logBlock LogBlockCollection
	// Receive
	receivedBlock *model.PbftBlock
	changeSig     chan model.States
}

func (pbft *PBFT) ChangeState(s model.States) {
	pbft.logger.Debugf("状态从%s 转换为%s",
		model.States_name[int32(pbft.sm.state)], model.States_name[int32(s)])
	if s == model.States_NotStartd || s == model.States_ViewChanging {
		pbft.sm.receivedBlock = nil
		// pbft.sm.logBlock.ResetBlock(pbft.ws.BlockNum + 1)
	}
	if s == model.States_NotStartd {
		if !pbft.stateTimeout.Stop() {
			select {
			case <-pbft.stateTimeout.C: // try to drain the channel
			default:
			}
		}
	}

	if s == pbft.sm.state {
		return
	}

	pbft.sm.Lock()
	pbft.sm.state = s
	pbft.sm.Unlock()
	if s != model.States_NotStartd && s != model.States_ViewChanging {
		if !pbft.stateTimeout.Stop() {
			select {
			case <-pbft.stateTimeout.C: // try to drain the channel
			default:
			}
		}
		pbft.stateTimeout.Reset(10 * time.Second)
		pbft.logger.Debugf("重置超时...")
	}
}

func (pbft *PBFT) CurrentState() model.States {
	return pbft.sm.state
}

func NewStateMachine() *StateMachine {
	return &StateMachine{
		state: model.States_NotStartd,
		// logMsg:    make(map[uint64]LogGroupByType),
		// logBlock:  make(LogBlockCollection),
		changeSig: make(chan model.States, 1),
	}
}

//Migrate  状态转移
func (pbft *PBFT) StateMigrate(msg *model.PbftMessage) {
	if msg != nil && !pbft.VerfifyMsg(msg) {
		pbft.logger.Warnf("接收到无效的msg")
		return
	} else {
		ok := pbft.AppendMsg(msg)
		if !ok {
			// 说明消息已经追加过 不在触发状态处理
			return
		}
	}

	curState := pbft.CurrentState()
	switch curState {
	case model.States_NotStartd:
		// 处于此状态 期望接收到 新区块提议
		defer pbft.statepollingTimer.AdjustmentPolling(normalDuraton)
		msgBysigners := pbft.FindStateMsg(pbft.ws.BlockNum+1, pbft.ws.View, model.MessageType_NewBlockProposal)
		if len(msgBysigners) == 0 {
			pbft.logger.Debugf("当前状态为%s 暂未收到%s类型消息",
				model.States_name[int32(curState)], model.MessageType_name[int32(model.MessageType_NewBlockProposal)])
			return
		}

		// 根据当前状态 判断是否是 primary Verifier
		if pbft.IsPrimaryVerfier() {
			pbft.logger.Debugf("当前状态为 %s, 提议的新区块为: %d, 视图编号为: %d, 当前节点为主验证节点 ",
				model.States_name[int32(curState)], pbft.ws.BlockNum+1, pbft.ws.View)

			// 检查之前是否已经广播过PrePrepare消息
			// prePrepareMsg := pbft.FindStateMsgBySinger(pbft.ws.BlockNum+1, pbft.ws.View, model.MessageType_PrePrepare, pbft.ws.CurVerfier.PublickKey)
			// if prePrepareMsg != nil {
			// 	if prePrepareMsg.Broadcast == false {
			// 		// 广播消息
			// 		pbft.AddBroadcastTask(prePrepareMsg)
			// 		// 直接迁移到 prepare状态
			// 		pbft.ChangeState(model.States_Preparing)
			// 		// 主动触发状态迁移
			// 		// pbft.Msgs.InsertMsg(signedMsg)
			// 	}
			// }

			// 尝试打包一个区块
			blk, err := pbft.packageBlock()
			if err != nil {
				pbft.logger.Errorf("当前状态为 %s, 准备打包新区块时发生了错误 err: %v",
					model.States_name[int32(curState)], err)
				return
			}
			// 向所有验证者发起pre-prepare 消息
			newMsg := model.PbftGenericMessage{
				Info: &model.PbftMessageInfo{MsgType: model.MessageType_PrePrepare,
					View: pbft.ws.View, SeqNum: pbft.ws.BlockNum + 1,
					SignerId: pbft.ws.CurVerfier.PublickKey,
					Sign:     nil,
				},
				Block: blk,
			}
			// 签名
			signedMsg, err := pbft.SignMsg(model.NewPbftMessage(&newMsg))
			if err != nil {
				pbft.logger.Warnf("当前状态为 %s, 发起pre-prepare消息时 在签名过程中发生错误 err: %v ",
					model.States_name[int32(curState)], err)
				return
			}

			// 广播消息
			pbft.broadcastStateMsg(signedMsg)
			// 直接迁移到 prepare状态
			pbft.ChangeState(model.States_Preparing)
			// 主动触发状态迁移
			pbft.Msgs.InsertMsg(signedMsg)

		} else {
			// 如果此次自己不是主验证者 切换到pre-prepare状态 开启超时 等待接收pre-prepare消息
			pbft.logger.Debugf("当前状态为 %s, 提议的新区块为: %d, 视图编号为: %d, 当前节点为副本验证节点 转换到pre-prepareing状态",
				model.States_name[int32(curState)], pbft.ws.BlockNum+1, pbft.ws.View)
			pbft.ChangeState(model.States_PrePreparing)
		}

	case model.States_PrePreparing:
		defer pbft.statepollingTimer.AdjustmentPolling(normalDuraton)
		// 此状态需要接收到 主节点发送的MessageType_PrePrepare
		// msgBysigners := pbft.FindStateMsg(pbft.ws.BlockNum+1, pbft.ws.View, model.MessageType_PrePrepare)
		primary := (pbft.ws.BlockNum + 1 + pbft.ws.View) % uint64(len(pbft.ws.Verifiers))
		if len(pbft.ws.Verifiers) == 1 {
			primary = 0
		}
		primarySigner := pbft.ws.Verifiers[primary].PublickKey
		preprepareMsg := pbft.FindStateMsgBySinger(pbft.ws.BlockNum+1, pbft.ws.View, model.MessageType_PrePrepare, primarySigner)
		if preprepareMsg == nil {
			pbft.logger.Debugf("当前状态为%s 暂未收到%s类型消息",
				model.States_name[int32(curState)], model.MessageType_name[int32(model.MessageType_PrePrepare)])
			return
		}
		// 检查是否已经广播过
		if !preprepareMsg.Broadcast {
			// 如果没有广播 帮住广播一次
			pbft.AddBroadcastTask(preprepareMsg)
		}

		// 执行到此处 说明收到了正确的由主节点发送的pre-prepare消息
		// 如果收到区块　校验区块　如果校验成功　则加入自己的签名 如果已经签名 并且数量已经等于2f+1 则不再广播blk
		var broadcastBlk *model.PbftBlock
		blk := pbft.FindBlock(pbft.ws.BlockNum+1, pbft.ws.View)
		// 对blk签名
		if blk != nil {
			signed := false
			for i := range blk.SignPairs {
				if bytes.Compare(blk.SignPairs[i].SignerId, pbft.ws.CurVerfier.PublickKey) == 0 {
					signed = true
					break
				}
				if !signed && bytes.Compare(blk.SignerId, pbft.ws.CurVerfier.PublickKey) == 0 {
					signed = true
				}
			}

			switch {
			case signed == true && len(blk.SignPairs)+1 < pbft.minNodeNum():
				broadcastBlk = blk
			case signed == false:
				b, err := pbft.signBlock(blk)
				if err != nil {
					pbft.logger.Warnf("当前节点处于PrePreparing 对区块进行签名是发生错误 err: %v",
						err)
					return
				}
				broadcastBlk = b
			case signed == true && len(blk.SignPairs)+1 >= pbft.minNodeNum():
				pbft.sm.receivedBlock = blk
			}
		}

		// 切换到prepare状态 同时添加广播 prepare消息
		newMsg := model.PbftGenericMessage{
			Info: &model.PbftMessageInfo{MsgType: model.MessageType_Prepare,
				View: pbft.ws.View, SeqNum: pbft.ws.BlockNum + 1,
				SignerId: pbft.ws.CurVerfier.PublickKey,
				Sign:     nil,
			},
			Block: broadcastBlk,
		}

		// 签名
		signedMsg, err := pbft.SignMsg(model.NewPbftMessage(&newMsg))
		if err != nil {
			pbft.logger.Warnf("当前状态为 %s, 发起prepare消息时 在签名过程中发生错误 err: %v ",
				model.States_name[int32(curState)], err)
			return
		}

		// 广播消息
		// pbft.AddBroadcastTask(signedMsg)
		pbft.ChangeState(model.States_Preparing)
		pbft.Msgs.InsertMsg(signedMsg)

	case model.States_Preparing:
		//检查自己是否已经发起prepare消息 如果没有 则发起
		prepareMsg := pbft.FindStateMsgBySinger(pbft.ws.BlockNum+1, pbft.ws.View, model.MessageType_Prepare, pbft.ws.CurVerfier.PublickKey)
		if prepareMsg != nil {
			// 说明之前已经广播过
			if !prepareMsg.Broadcast {
				pbft.AddBroadcastTask(prepareMsg)
			}
		}

		// 此状态需要接收足够多的prepare消息 方能迁移成功 在这个状态 等待接收足够多的prepare消息
		msgBysigners := pbft.FindStateMsg(pbft.ws.BlockNum+1, pbft.ws.View, model.MessageType_Prepare)
		if len(msgBysigners) == 0 {
			pbft.logger.Debugf("当前状态为%s 暂未收到%s类型消息",
				model.States_name[int32(curState)], model.MessageType_name[int32(model.MessageType_Prepare)])

			pbft.statepollingTimer.AdjustmentPolling(normalDuraton)
			return
		}

		if pbft.sm.receivedBlock == nil {
			blk := pbft.FindBlock(pbft.ws.BlockNum+1, pbft.ws.View)
			if blk != nil && len(blk.SignPairs)+1 >= pbft.minNodeNum() {
				pbft.sm.receivedBlock = blk
			}
		}
		if len(msgBysigners) >= pbft.minNodeNum() {
			// 收到了足够多的prepare 切换到下一个状态
			// 满足节点数量  进入checking
			pbft.ChangeState(model.States_Checking)
			//  加快进入下一个状态处理
			pbft.statepollingTimer.AdjustmentPolling(fastDuration)
		} else {
			pbft.logger.Debugf("当前状态为%s 暂未收到足够多的%s类型消息",
				model.States_name[int32(curState)], model.MessageType_name[int32(model.MessageType_Prepare)])
			pbft.statepollingTimer.AdjustmentPolling(normalDuraton)
			return
		}

	case model.States_Checking:
		defer pbft.statepollingTimer.AdjustmentPolling(normalDuraton)
		// 在这个状态 等待收到足够签名的区块 如果收到 则直接进入下一个状态 否则 等待
		if pbft.sm.receivedBlock == nil {
			blk := pbft.FindBlock(pbft.ws.BlockNum+1, pbft.ws.View)
			if blk != nil && len(blk.SignPairs)+1 >= pbft.minNodeNum() {
				pbft.sm.receivedBlock = blk
			}
		}

		if pbft.sm.receivedBlock != nil {
			// 说明已经收到提交的区块 并且验证通过
			newMsg := &model.PbftGenericMessage{
				Info: &model.PbftMessageInfo{MsgType: model.MessageType_Commit,
					View: pbft.ws.View, SeqNum: pbft.ws.BlockNum + 1,
					SignerId: pbft.ws.CurVerfier.PublickKey,
					Sign:     nil,
				},
			}
			signedMsg, err := pbft.SignMsg(model.NewPbftMessage(newMsg))
			if err != nil {
				pbft.logger.Debugf("当前状态为 %s, 发起commit消息时 在签名过程中发生错误 err: %v ",
					model.States_name[int32(curState)], err)
				return
			}
			// pbft.AddBroadcastTask(signedMsg)
			pbft.ChangeState(model.States_Committing)
			pbft.Msgs.InsertMsg(signedMsg)
		}

	case model.States_Committing:
		// 检查自己是否已经广播 如果未广播 则广播
		commitMsg := pbft.FindStateMsgBySinger(pbft.ws.BlockNum+1, pbft.ws.View, model.MessageType_Commit, pbft.ws.CurVerfier.PublickKey)
		if commitMsg != nil {
			if !commitMsg.Broadcast {
				pbft.AddBroadcastTask(commitMsg)
			}
		}
		msgBysigners := pbft.FindStateMsg(pbft.ws.BlockNum+1, pbft.ws.View, model.MessageType_Commit)
		if len(msgBysigners) == 0 {
			pbft.logger.Debugf("当前状态为%s 暂未收到%s类型消息",
				model.States_name[int32(curState)], model.MessageType_name[int32(model.MessageType_Commit)])
			pbft.statepollingTimer.AdjustmentPolling(normalDuraton)
			return
		}

		if len(msgBysigners) >= pbft.minNodeNum() {
			// 说明已经收到了足够多的commit消息 迁移到finish状态 进行commit区块
			pbft.ChangeState(model.States_Finished)
			pbft.statepollingTimer.AdjustmentPolling(fastDuration)
		}
		// 广播消息
		// pbft.AddBroadcastTask(signedMsg)
		// pbft.Msgs.InsertMsg(signedMsg)

	case model.States_Finished:
		// 停止超时定时器
		// 重放区块
		// 切换到not start
		pbft.CommitBlock(pbft.sm.receivedBlock)
		pbft.ChangeState(model.States_NotStartd)

	case model.States_ViewChanging:
		viewChangeMsg := pbft.FindStateMsgBySinger(pbft.ws.BlockNum+1, pbft.ws.View, model.MessageType_ViewChange, pbft.ws.CurVerfier.PublickKey)
		if viewChangeMsg != nil {
			if !viewChangeMsg.Broadcast {
				pbft.AddBroadcastTask(viewChangeMsg)
			}
		}
		msgBysigners := pbft.FindStateMsg(pbft.ws.BlockNum+1, pbft.ws.View, model.MessageType_ViewChange)
		if len(msgBysigners) == 0 {
			pbft.logger.Debugf("当前状态为%s 暂未收到%s类型消息",
				model.States_name[int32(curState)], model.States_name[int32(model.States_ViewChanging)])
			return
		}
		if len(msgBysigners) >= pbft.minNodeNum() {
			// 满足节点数量   进入not start view +1
			pbft.ws.IncreaseView()
			pbft.ChangeState(model.States_NotStartd)
		}
	}
}

func (pbft *PBFT) minNodeNum() int {
	f := len(pbft.ws.Verifiers) / 3
	var minNodes int
	if f == 0 {
		minNodes = len(pbft.ws.Verifiers)
	} else {
		minNodes = 2*f + 1
	}
	return minNodes
}
