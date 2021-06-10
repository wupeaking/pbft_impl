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
	// 区块编号  <---->  接收到的消息日志 (消息类型-视图编号: 消息)
	logMsg LogMsgCollection
	// Receive
	receivedBlock *model.PbftBlock
	changeSig     chan model.States
}

func (pbft *PBFT) ChangeState(s model.States) {
	if s == model.States_NotStartd || s == model.States_ViewChanging {
		pbft.sm.receivedBlock = nil
	}

	if s == pbft.sm.state {
		return
	}

	pbft.sm.Lock()
	pbft.sm.state = s
	pbft.sm.Unlock()
	if s != model.States_NotStartd && s != model.States_ViewChanging {
		pbft.timer.Reset(10 * time.Second)
	}
}

func (pbft *PBFT) CurrentState() model.States {
	return pbft.sm.state
}

func NewStateMachine() *StateMachine {
	return &StateMachine{
		state:     model.States_NotStartd,
		logMsg:    make(map[uint64]LogGroupByType),
		changeSig: make(chan model.States, 1),
	}
}

//Migrate  状态转移
func (pbft *PBFT) StateMigrate(msg *model.PbftMessage) {
	pbft.logger.Debugf("进入状态转移函数, 当前状态: %v", pbft.sm.state)
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
			pbft.logger.Warnf("当前状态为%s 暂未收到%s类型消息",
				model.States_name[int32(curState)], model.States_name[int32(model.MessageType_NewBlockProposal)])
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
				pbft.logger.Debugf("当前状态为 %s, 发起pre-prepare消息时 在签名过程中发生错误 err: %v ",
					model.States_name[int32(curState)], err)
				return
			}
			// pbft.sm.receivedBlock = blk
			// 广播消息
			// pbft.broadcastStateMsg(signedMsg)
			pbft.AddBroadcastTask(signedMsg)
			// 直接迁移到 prepare状态
			pbft.ChangeState(model.States_Preparing)
			// pbft.appendLogMsg(signedMsg)

		} else {
			// 如果此次自己不是主验证者 切换到pre-prepare状态 开启超时 等待接收pre-prepare消息
			pbft.logger.Debugf("当前状态为 %s, 提议的新区块为: %d, 视图编号为: %d, 当前节点为副本验证节点 转换到pre-prepareing状态",
				model.States_name[int32(curState)], pbft.ws.BlockNum+1, pbft.ws.View)
			pbft.ChangeState(model.States_PrePreparing)
		}

	case model.States_PrePreparing:
		msgBysigners := pbft.sm.logMsg.FindMsg(pbft.ws.BlockNum+1, model.MessageType_PrePrepare, int(pbft.ws.View))
		if len(msgBysigners) == 0 {
			pbft.logger.Warnf("当前状态为%s 暂未收到%s类型消息",
				model.States_name[int32(curState)], model.States_name[int32(model.MessageType_PrePrepare)])
			return
		}
		// 说明收到了 pre-prepare消息 验证消息内容是否是期望的 验证签名结果是否正确 验证是否是主节点签名
		primary := (pbft.ws.BlockNum + 1 + pbft.ws.View) % uint64(len(pbft.ws.Verifiers))
		if len(pbft.ws.Verifiers) == 1 {
			primary = 0
		}
		// 验证是否是有由主节点发送的
		msg, ok := msgBysigners[string(pbft.ws.Verifiers[primary].PublickKey)]
		if !ok {
			pbft.logger.Warnf("当前节点处于PrePreparing 收到pre-prepare消息 但是 pre-prepare消息不是由主节点%d发出的",
				primary)
		}
		// 如果收到区块　校验区块　如果校验成功　则加入自己的签名
		var blk *model.PbftBlock
		if msg.block != nil && pbft.verfifyBlock(msg.block) {
			blk, err := pbft.signBlock(msg.block)
			if err != nil {
				pbft.logger.Warnf("当前节点处于PrePreparing　签名区块出错 err: %v",
					err)
				return
			}
			if pbft.VerfifyBlockHeader(blk) {
				// 说明２/3节点已经验证
				pbft.sm.receivedBlock = msg.block
			}
		}

		// 执行到此处 说明收到了正确的由主节点发送的pre-prepare消息
		// 广播prepare消息 切换到preparing状态  重置超时 等待接收足够多的prepare消息
		// 向所有验证者发起prepare 消息
		newMsg := model.PbftGenericMessage{
			Info: &model.PbftMessageInfo{MsgType: model.MessageType_Prepare,
				View: pbft.ws.View, SeqNum: pbft.ws.BlockNum + 1,
				SignerId: pbft.ws.CurVerfier.PublickKey,
				Sign:     nil, // todo:: 需要签名
			},
			Block: blk,
		}

		// 签名
		signedInfo, err := pbft.signMsgInfo(newMsg.Info)
		if err != nil {
			pbft.logger.Debugf("当前状态为 %s, 发起prepare消息时 在签名过程中发生错误 err: %v ",
				model.States_name[int32(curState)], err)
			return
		}
		newMsg.Info = signedInfo
		signedMsg := model.NewPbftMessage(&newMsg)
		// pbft.appendLogMsg(signedMsg)
		// 广播消息
		// pbft.broadcastStateMsg(signedMsg)
		pbft.AddBroadcastTask(signedMsg)
		pbft.ChangeState(model.States_Preparing)
		pbft.Msgs.InsertMsg(signedMsg)

	case model.States_Preparing:
		// 此状态需要接收足够多的prepare消息 方能迁移成功
		content := msg.GetGeneric()
		if content == nil {
			pbft.logger.Warnf("当前状态为%s 但是收到的消息类型为ViewChange", model.States_name[int32(curState)])
			return
		}
		msgType := content.Info.MsgType
		if msgType != model.MessageType_Prepare {
			// pbft.appendLogMsg(msg)
			pbft.logger.Warnf("当前节点处于Preparing 期待收到的消息类型为Prepare 此次消息类型为: %s, 忽略此次消息 ",
				model.MessageType_name[int32(msgType)])
			return
		}
		// 1.需要区块编号和本机对应 2.视图编号与本机对应 3.验证者处于验证者列表中 4.签名正确
		if content.Info.SeqNum != pbft.ws.BlockNum+1 {
			// pbft.appendLogMsg(msg)
			pbft.logger.Warnf("当前节点处于Preparing 收到prepare消息 但是区块编号和本地区块编号不一致, 收到的序号为: %d, 本地区块编号为: %d",
				content.Info.SeqNum, pbft.ws.BlockNum+1)
			return
		}
		if content.Info.View != pbft.ws.View {
			// pbft.appendLogMsg(msg)
			pbft.logger.Warnf("当前节点处于Preparing 收到prepare消息 但是视图编号与本地视图编号不一致, 收到的视图序号为: %d, 本地视图序号为: %d",
				content.Info.View, pbft.ws.View)
			return
		}

		// 计算fault 数量
		f := len(pbft.ws.Verifiers) / 3
		var minNodes int
		if f == 0 {
			minNodes = len(pbft.ws.Verifiers)
		} else {
			minNodes = 2*f + 1
		}
		nodes := make(map[string]bool)
		logMsg := pbft.sm.logMsg[pbft.ws.BlockNum+1]

		prepareMsgs := make([]*model.PbftMessageInfo, 0)
		var localprepareMsg *model.PbftMessageInfo

		for _, log := range logMsg {
			if log.MessageType == model.MessageType_Prepare && log.view == pbft.ws.View {
				// 1. 签名者在验证列表中
				privateKey, ok := pbft.verifiers[string(log.msg.GetGeneric().Info.SignerId)]
				if !ok {
					continue
				}
				{
					//todo:: 验证签名
					_ = privateKey
				}
				if bytes.Compare(log.msg.GetGeneric().Info.SignerId, pbft.ws.CurVerfier.PublickKey) == 0 {
					localprepareMsg = log.msg.GetGeneric().Info
				} else {
					prepareMsgs = append(prepareMsgs, log.msg.GetGeneric().Info)
					nodes[string(log.msg.GetGeneric().Info.SignerId)] = true
				}

			}
			// 去掉自己 只需要2f
			if len(nodes) >= minNodes-1 {
				// 满足节点数量  进入checking
				pbft.sm.ChangeState(model.States_Checking)
				// todo:: 如果本机已经保存了完整区块 广播区块
				pbft.timer.Reset(10 * time.Second)
				// 触发信号 迁移到checking状态
				// 为啥需要手动触发 是由于 可能这个消息已经被收到了 到这一直不会收到消息转移到checking状态
				// pbft.tiggerMigrate(model.States_Checking)
				return
			}
		}
		// 说明还没有达到2f+1 将已经收到的内容 广播出去 提高成功率
		var newMsg model.PbftGenericMessage
		if localprepareMsg == nil {
			newMsg.Info = &model.PbftMessageInfo{MsgType: model.MessageType_Prepare,
				View: pbft.ws.View, SeqNum: pbft.ws.BlockNum + 1,
				SignerId: pbft.ws.CurVerfier.PublickKey,
				Sign:     nil, // todo:: 需要签名
			}
			signedInfo, err := pbft.signMsgInfo(newMsg.Info)
			newMsg.Info = signedInfo
			if err != nil {
				pbft.logger.Debugf("当前状态为 %s, 发起prepare消息时 在签名过程中发生错误 err: %v ",
					model.States_name[int32(curState)], err)
				return
			}

		} else {
			newMsg.Info = localprepareMsg
		}
		newMsg.OtherInfos = prepareMsgs
		// 广播消息
		// pbft.broadcastStateMsg(model.NewPbftMessage(&newMsg))
		pbft.AddBroadcastTask(model.NewPbftMessage(&newMsg))

	case model.States_Checking:
		content := msg.GetGeneric()
		if content == nil && pbft.sm.receivedBlock == nil {
			pbft.logger.Warnf("当前状态为%s 但是收到的消息类型为ViewChange", model.States_name[int32(curState)])
			return
		}
		// 检查消息体中是否有有效的区块
		if content.Block == nil && pbft.sm.receivedBlock == nil {
			return
		}

		if pbft.sm.receivedBlock == nil && pbft.VerfifyMostBlock(content.Block) {
			pbft.sm.receivedBlock = content.Block
		}

		if pbft.sm.receivedBlock != nil {
			// 说明已经收到提交的区块 并且验证通过
			// 广播Commit消息
			newMsg := &model.PbftGenericMessage{
				Info: &model.PbftMessageInfo{MsgType: model.MessageType_Commit,
					View: pbft.ws.View, SeqNum: pbft.ws.BlockNum + 1,
					SignerId: pbft.ws.CurVerfier.PublickKey,
					Sign:     nil,
				},
				// Block: pbft.sm.receivedBlock,
			}
			signedInfo, err := pbft.signMsgInfo(newMsg.Info)
			if err != nil {
				pbft.logger.Debugf("当前状态为 %s, 发起commit消息时 在签名过程中发生错误 err: %v ",
					model.States_name[int32(curState)], err)
				return
			}
			newMsg.Info = signedInfo

			pbft.appendLogMsg(model.NewPbftMessage(newMsg))
			// pbft.broadcastStateMsg(model.NewPbftMessage(newMsg))
			pbft.AddBroadcastTask(model.NewPbftMessage(newMsg))
			pbft.sm.ChangeState(model.States_Committing)
			pbft.timer.Reset(10 * time.Second)
			return
		}

	case model.States_Committing:
		content := msg.GetGeneric()
		if content == nil {
			pbft.logger.Warnf("当前状态为%s 但是收到的消息类型为ViewChange", model.States_name[int32(curState)])
			return
		}
		msgType := content.Info.MsgType
		if msgType != model.MessageType_Commit {
			pbft.logger.Warnf("当前节点处于Commiting 期待收到的消息类型为Commit 此次消息类型为: %s, 忽略此次消息 ",
				model.MessageType_name[int32(msgType)])
			return
		}
		// 1.需要区块编号和本机对应 2.视图编号与本机对应 3.验证者处于验证者列表中 4.签名正确
		if content.Info.SeqNum != pbft.ws.BlockNum+1 {
			pbft.logger.Warnf("当前节点处于Commiting 收到Commit消息 但是区块编号和本地区块编号不一致, 收到的序号为: %d, 本地区块编号为: %d",
				content.Info.SeqNum, pbft.ws.BlockNum+1)
			return
		}
		if content.Info.View != pbft.ws.View {
			// pbft.appendLogMsg(msg)
			pbft.logger.Warnf("当前节点处于Commiting 收到Commit消息 但是视图编号与本地视图编号不一致, 收到的视图序号为: %d, 本地视图序号为: %d",
				content.Info.View, pbft.ws.View)
			return
		}

		// 计算fault 数量
		f := len(pbft.ws.Verifiers) / 3
		var minNodes int
		if f == 0 {
			minNodes = len(pbft.ws.Verifiers)
		} else {
			minNodes = 2*f + 1
		}
		nodes := make(map[string]bool)
		logMsg := pbft.sm.logMsg[pbft.ws.BlockNum+1]

		commitMsgs := make([]*model.PbftMessageInfo, 0)
		var localcommitMsg *model.PbftMessageInfo
		for _, log := range logMsg {
			if log.MessageType == model.MessageType_Commit && log.view == pbft.ws.View {
				if bytes.Compare(log.msg.GetGeneric().Info.SignerId, pbft.ws.CurVerfier.PublickKey) == 0 {
					localcommitMsg = log.msg.GetGeneric().Info
				} else {
					commitMsgs = append(commitMsgs, log.msg.GetGeneric().Info)
					nodes[string(log.msg.GetGeneric().Info.SignerId)] = true
				}
			}
			if len(nodes) >= minNodes-1 {
				// 满足节点数量  进入checking
				pbft.sm.ChangeState(model.States_Finished)
				pbft.timer.Reset(10 * time.Second)
				// 触发信号 迁移到checking状态
				// 为啥需要手动触发 是由于 可能这个消息已经被收到了 到这一直不会收到消息转移到checking状态
				//pbft.tiggerMigrate(model.States_Finished)
				return
			}
		}
		// 说明还没有达到2f+1 将已经收到的内容 广播出去 提高成功率
		var newMsg model.PbftGenericMessage
		if localcommitMsg == nil {
			newMsg.Info = &model.PbftMessageInfo{MsgType: model.MessageType_Prepare,
				View: pbft.ws.View, SeqNum: pbft.ws.BlockNum + 1,
				SignerId: pbft.ws.CurVerfier.PublickKey,
				Sign:     nil, // todo:: 需要签名
			}
			signedInfo, err := pbft.signMsgInfo(newMsg.Info)
			newMsg.Info = signedInfo
			if err != nil {
				pbft.logger.Debugf("当前状态为 %s, 发起commit消息时 在签名过程中发生错误 err: %v ",
					model.States_name[int32(curState)], err)
				return
			}
		} else {
			newMsg.Info = localcommitMsg
		}
		newMsg.OtherInfos = commitMsgs
		// 广播消息
		// pbft.broadcastStateMsg(model.NewPbftMessage(&newMsg))
		pbft.AddBroadcastTask(model.NewPbftMessage(&newMsg))

	case model.States_Finished:
		// 停止超时定时器
		// 重放区块
		// 切换到not start
		pbft.timer.Stop()
		pbft.CommitBlock(pbft.sm.receivedBlock)
		pbft.sm.ChangeState(model.States_NotStartd)

	case model.States_ViewChanging:
		content := msg.GetViewChange()
		if content == nil {
			pbft.logger.Warnf("当前状态为%s 但是收到的消息类型为GenicMessage", model.States_name[int32(curState)])
			return
		}
		msgType := content.Info.MsgType
		if msgType != model.MessageType_ViewChange {
			pbft.logger.Warnf("当前节点处于ViewChanging 期待收到的消息类型为ViewChanging  此次消息类型为: %s, 忽略此次消息 ",
				model.MessageType_name[int32(msgType)])
			return
		}
		// 1.需要区块编号和本机对应 2.视图编号与本机对应 3.验证者处于验证者列表中 4.签名正确
		if content.Info.SeqNum != pbft.ws.BlockNum+1 {
			return
		}
		if content.Info.View != pbft.ws.View {
			return
		}

		// 计算fault 数量
		f := len(pbft.ws.Verifiers) / 3
		var minNodes int
		if f == 0 {
			minNodes = len(pbft.ws.Verifiers)
		} else {
			minNodes = 2*f + 1
		}
		nodes := make(map[string]bool)
		logMsg := pbft.sm.logMsg[pbft.ws.BlockNum+1]

		viewMsgs := make([]*model.PbftMessageInfo, 0)
		// var localviewMsg *model.PbftMessageInfo
		for _, log := range logMsg {
			if log.MessageType == model.MessageType_ViewChange && log.view == pbft.ws.View {
				if bytes.Compare(log.msg.GetViewChange().Info.SignerId, pbft.ws.CurVerfier.PublickKey) == 0 {
					// localviewMsg = log.msg.GetViewChange().Info
				} else {
					viewMsgs = append(viewMsgs, log.msg.GetViewChange().Info)
					nodes[string(log.msg.GetViewChange().Info.SignerId)] = true
				}

			}
			if len(nodes) >= minNodes-1 {
				// 满足节点数量   进入not start view +1
				pbft.ws.IncreaseView()
				pbft.sm.ChangeState(model.States_NotStartd)
				// pbft.timer.Reset(10 * time.Second)
				return
			}
		}
	}
}
