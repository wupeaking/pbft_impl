package consensus

import (
	"sync"
	"time"

	"github.com/wupeaking/pbft_impl/model"
)

type StateMachine struct {
	state model.States
	sync.Mutex
	// 区块编号  <---->  接收到的消息日志 (消息类型-视图编号: 消息)
	logMsg   LogMsgCollection
	logBlock LogBlockCollection
	// Receive
	receivedBlock *model.PbftBlock
	changeSig     chan model.States
}

func (pbft *PBFT) ChangeState(s model.States) {
	pbft.logger.Debugf("状态从%s 转换为%s",
		model.States_name[int32(pbft.sm.state)], model.States_name[int32(s)])
	if s == model.States_NotStartd || s == model.States_ViewChanging {
		pbft.sm.receivedBlock = nil
		pbft.sm.logBlock.ResetBlock(pbft.ws.BlockNum + 1)
	}
	if s == model.States_NotStartd {
		if !pbft.timer.Stop() {
			select {
			case <-pbft.timer.C: // try to drain the channel
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
		if !pbft.timer.Stop() {
			select {
			case <-pbft.timer.C: // try to drain the channel
			default:
			}
		}
		pbft.timer.Reset(10 * time.Second)
		pbft.logger.Debugf("重置超时...")
	}
}

func (pbft *PBFT) CurrentState() model.States {
	return pbft.sm.state
}

func NewStateMachine() *StateMachine {
	return &StateMachine{
		state:     model.States_NotStartd,
		logMsg:    make(map[uint64]LogGroupByType),
		logBlock:  make(LogBlockCollection),
		changeSig: make(chan model.States, 1),
	}
}

//Migrate  状态转移
func (pbft *PBFT) StateMigrate(msg *model.PbftMessage) {

	if !pbft.VerfifyMsg(msg) {
		pbft.logger.Warnf("接收到无效的msg")
		return
	}
	pbft.appendLogMsg(msg)

	curState := pbft.CurrentState()
	switch curState {
	case model.States_NotStartd:
		// 处于此状态 期望接收到 新区块提议
		msgBysigners := pbft.sm.logMsg.FindMsg(pbft.ws.BlockNum+1, model.MessageType_NewBlockProposal, int(pbft.ws.View))
		if len(msgBysigners) == 0 {
			pbft.logger.Debugf("当前状态为%s 暂未收到%s类型消息",
				model.States_name[int32(curState)], model.MessageType_name[int32(model.MessageType_NewBlockProposal)])
			return
		}

		// 根据当前状态 判断是否是 primary Verifier
		if pbft.IsPrimaryVerfier() {
			pbft.logger.Debugf("当前状态为 %s, 提议的新区块为: %d, 视图编号为: %d, 当前节点为主验证节点 ",
				model.States_name[int32(curState)], pbft.ws.BlockNum+1, pbft.ws.View)

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
			pbft.AddBroadcastTask(signedMsg)
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
		msgBysigners := pbft.sm.logMsg.FindMsg(pbft.ws.BlockNum+1, model.MessageType_PrePrepare, int(pbft.ws.View))
		if len(msgBysigners) == 0 {
			pbft.logger.Debugf("当前状态为%s 暂未收到%s类型消息",
				model.States_name[int32(curState)], model.MessageType_name[int32(model.MessageType_PrePrepare)])
			return
		}
		// 说明收到了 pre-prepare消息 验证消息内容是否是期望的 验证签名结果是否正确 验证是否是主节点签名
		primary := (pbft.ws.BlockNum + 1 + pbft.ws.View) % uint64(len(pbft.ws.Verifiers))
		if len(pbft.ws.Verifiers) == 1 {
			primary = 0
		}
		// 验证是否是有由主节点发送的
		_, ok := msgBysigners[string(pbft.ws.Verifiers[primary].PublickKey)]
		if !ok {
			pbft.logger.Warnf("当前节点处于PrePreparing 收到pre-prepare消息 但是 pre-prepare消息不是由主节点%d发出的",
				primary)
			return
		}

		// 执行到此处 说明收到了正确的由主节点发送的pre-prepare消息
		// 如果收到区块　校验区块　如果校验成功　则加入自己的签名
		blks := pbft.sm.logBlock.FindBlock(pbft.ws.BlockNum + 1)
		maxNum := 0
		var blk *model.PbftBlock
		for num, b := range blks {
			if num >= maxNum {
				maxNum = num
				blk = b
			}
		}

		// 对blk签名
		blk, err := pbft.signBlock(blk)
		if err != nil {
			pbft.logger.Warnf("当前节点处于PrePreparing 对区块进行签名是发生错误 err: %v",
				err)
			return
		}
		// 广播prepare消息 切换到preparing状态  重置超时 等待接收足够多的prepare消息
		// 向所有验证者发起prepare 消息
		newMsg := model.PbftGenericMessage{
			Info: &model.PbftMessageInfo{MsgType: model.MessageType_Prepare,
				View: pbft.ws.View, SeqNum: pbft.ws.BlockNum + 1,
				SignerId: pbft.ws.CurVerfier.PublickKey,
				Sign:     nil,
			},
			Block: blk,
		}

		// 签名
		signedMsg, err := pbft.SignMsg(model.NewPbftMessage(&newMsg))
		if err != nil {
			pbft.logger.Warnf("当前状态为 %s, 发起prepare消息时 在签名过程中发生错误 err: %v ",
				model.States_name[int32(curState)], err)
			return
		}

		// 广播消息
		pbft.AddBroadcastTask(signedMsg)
		pbft.ChangeState(model.States_Preparing)
		pbft.Msgs.InsertMsg(signedMsg)

	case model.States_Preparing:
		// 此状态需要接收足够多的prepare消息 方能迁移成功
		msgBysigners := pbft.sm.logMsg.FindMsg(pbft.ws.BlockNum+1, model.MessageType_Prepare, int(pbft.ws.View))
		if len(msgBysigners) == 0 {
			pbft.logger.Debugf("当前状态为%s 暂未收到%s类型消息",
				model.States_name[int32(curState)], model.MessageType_name[int32(model.MessageType_Prepare)])
			return
		}
		// 如果收到区块　校验区块　如果校验成功　则加入自己的签名
		blks := pbft.sm.logBlock.FindBlock(pbft.ws.BlockNum + 1)
		maxNum := 0
		var blk *model.PbftBlock
		for num, b := range blks {
			if num >= maxNum {
				maxNum = num
				blk = b
			}
		}
		var newMsg model.PbftGenericMessage
		newMsg.Info = &model.PbftMessageInfo{MsgType: model.MessageType_Prepare,
			View: pbft.ws.View, SeqNum: pbft.ws.BlockNum + 1,
			SignerId: pbft.ws.CurVerfier.PublickKey,
			Sign:     nil,
		}
		newMsg.Block = blk

		signedMsg, err := pbft.SignMsg(model.NewPbftMessage(&newMsg))
		if err != nil {
			pbft.logger.Debugf("当前状态为 %s, 发起prepare消息时 在签名过程中发生错误 err: %v ",
				model.States_name[int32(curState)], err)
			return
		}
		// 广播消息
		pbft.AddBroadcastTask(signedMsg)
		if len(msgBysigners) >= pbft.minNodeNum() {
			// 满足节点数量  进入checking
			pbft.ChangeState(model.States_Checking)
		}
		// 触发迁移
		pbft.Msgs.InsertMsg(signedMsg)

	case model.States_Checking:
		// 如果收到区块　校验区块　如果校验成功　则加入自己的签名
		blks := pbft.sm.logBlock.FindBlock(pbft.ws.BlockNum + 1)
		maxNum := 0
		var blk *model.PbftBlock
		for num, b := range blks {
			if num >= maxNum {
				maxNum = num
				blk = b
			}
		}
		if maxNum+1 >= pbft.minNodeNum() {
			pbft.sm.receivedBlock = blk
		}
		// 检查是否搜集到足够签名的区块
		if pbft.sm.receivedBlock != nil {
			// 说明已经收到提交的区块 并且验证通过
			// 广播Commit消息
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
			pbft.AddBroadcastTask(signedMsg)
			pbft.ChangeState(model.States_Committing)
			pbft.Msgs.InsertMsg(signedMsg)
		}

	case model.States_Committing:
		// 计算fault 数量
		f := len(pbft.ws.Verifiers) / 3
		var minNodes int
		if f == 0 {
			minNodes = len(pbft.ws.Verifiers)
		} else {
			minNodes = 2*f + 1
		}
		msgBysigners := pbft.sm.logMsg.FindMsg(pbft.ws.BlockNum+1, model.MessageType_Commit, int(pbft.ws.View))
		if len(msgBysigners) == 0 {
			pbft.logger.Debugf("当前状态为%s 暂未收到%s类型消息",
				model.States_name[int32(curState)], model.MessageType_name[int32(model.MessageType_Commit)])
			return
		}

		var newMsg model.PbftGenericMessage
		newMsg.Info = &model.PbftMessageInfo{MsgType: model.MessageType_Commit,
			View: pbft.ws.View, SeqNum: pbft.ws.BlockNum + 1,
			SignerId: pbft.ws.CurVerfier.PublickKey,
			Sign:     nil,
		}
		signedMsg, err := pbft.SignMsg(model.NewPbftMessage(&newMsg))
		if err != nil {
			pbft.logger.Debugf("当前状态为 %s, 发起commit消息时 在签名过程中发生错误 err: %v ",
				model.States_name[int32(curState)], err)
			return
		}
		if len(msgBysigners) >= minNodes {
			// 满足节点数量  进入checking
			pbft.ChangeState(model.States_Finished)
		}
		// 广播消息
		pbft.AddBroadcastTask(signedMsg)
		pbft.Msgs.InsertMsg(signedMsg)

	case model.States_Finished:
		// 停止超时定时器
		// 重放区块
		// 切换到not start
		pbft.CommitBlock(pbft.sm.receivedBlock)
		pbft.ChangeState(model.States_NotStartd)

	case model.States_ViewChanging:
		msgBysigners := pbft.sm.logMsg.FindMsg(pbft.ws.BlockNum+1, model.MessageType_ViewChange, int(pbft.ws.View))
		if len(msgBysigners) == 0 {
			pbft.logger.Debugf("当前状态为%s 暂未收到%s类型消息",
				model.States_name[int32(curState)], model.States_name[int32(model.MessageType_Commit)])
			return
		}
		var newMsg model.PbftViewChange
		newMsg.Info = &model.PbftMessageInfo{MsgType: model.MessageType_ViewChange,
			View: pbft.ws.View, SeqNum: pbft.ws.BlockNum + 1,
			SignerId: pbft.ws.CurVerfier.PublickKey,
			Sign:     nil,
		}
		signedMsg, err := pbft.SignMsg(model.NewPbftMessage(&newMsg))
		if err != nil {
			pbft.logger.Debugf("当前状态为 %s, 发起commit消息时 在签名过程中发生错误 err: %v ",
				model.States_name[int32(curState)], err)
			return
		}
		if len(msgBysigners) >= pbft.minNodeNum() {
			// 满足节点数量   进入not start view +1
			pbft.ws.IncreaseView()
			pbft.ChangeState(model.States_NotStartd)
		}
		// 广播消息
		pbft.AddBroadcastTask(signedMsg)
		pbft.Msgs.InsertMsg(signedMsg)
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
