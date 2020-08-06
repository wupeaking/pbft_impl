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
	// 区块编号  <---->  接收到的消息日志
	logMsg map[uint64][]LogMessage
}

type LogMessage struct {
	model.MessageType
	msg   *model.PbftMessage
	block *model.PbftBlock
	view  uint64
}

func (pbft *PBFT) appendLogMsg(msg *model.PbftMessage) {
	content := msg.GetGeneric()
	if content != nil {
		for i := range content.OtherInfos {
			pbft.appendLogMsg(model.NewPbftMessage(&model.PbftGenericMessage{Info: content.OtherInfos[i]}))
		}
		logMsgs := pbft.sm.logMsg[content.Info.SeqNum]
		logMsgs = append(logMsgs, LogMessage{
			MessageType: content.Info.MsgType,
			msg:         msg,
			view:        content.Info.View,
			block:       content.Block,
		})
		return
	}
	if content := msg.GetViewChange(); content != nil {
		logMsgs := pbft.sm.logMsg[content.Info.SeqNum]
		logMsgs = append(logMsgs, LogMessage{
			MessageType: content.Info.MsgType,
			msg:         msg,
			view:        content.Info.View,
		})
	}
}

func (sm *StateMachine) ChangeState(s model.States) {
	sm.Lock()
	sm.state = s
	sm.Unlock()
}

func (sm *StateMachine) CurrentState() model.States {
	return sm.state
}

func NewStateMachine() *StateMachine {
	return &StateMachine{
		state: model.States_NotStartd,
	}
}

//Migrate  状态转移
func (pbft *PBFT) StateMigrate(msg *model.PbftMessage) {
	if !pbft.VerfifyMsg(msg) {
		pbft.logger.Warnf("接收到无效的msg")
		return
	}

	pbft.appendLogMsg(msg)
	curState := pbft.sm.CurrentState()
	switch curState {
	case model.States_NotStartd:
		// 处于此状态 期望接收到 新区块提议
		content := msg.GetGeneric()
		if content == nil {
			pbft.logger.Warnf("当前状态为%s 但是收到的消息类型为ViewChange", model.States_name[int32(curState)])
			return
		}
		msgType := content.Info.MsgType
		if msgType != model.MessageType_NewBlockProposal {
			pbft.logger.Warnf("当前状态为%s 期待收到新区块提议 但是收到的消息类型为%s",
				model.States_name[int32(curState)], model.MessageType_name[int32(msgType)])
			return
		}

		// 根据当前状态 判断是否是 primary Verifier
		primary := (pbft.ws.BlockNum + 1 + pbft.ws.View) % uint64(len(pbft.ws.Verifiers))
		if primary == uint64(pbft.ws.VerifierNo) {
			pbft.logger.Debugf("当前状态为 %s, 提议的新区块为: %d, 视图编号为: %d, 当前节点为主验证节点 ",
				model.States_name[int32(curState)], pbft.ws.BlockNum+1, pbft.ws.View)
			// 向所有验证者发起pre-prepare 消息
			newMsg := model.PbftGenericMessage{
				Info: &model.PbftMessageInfo{MsgType: model.MessageType_PrePrepare,
					View: pbft.ws.View, SeqNum: pbft.ws.BlockNum + 1,
					SignerId: pbft.ws.CurVerfier.PublickKey,
					Sign:     nil,
				},
			}
			// 签名
			signedMsg, err := pbft.SignMsg(model.NewPbftMessage(newMsg))
			if err != nil {
				pbft.logger.Debugf("当前状态为 %s, 发起pre-prepare消息时 在签名过程中发生错误 err: %v ",
					model.States_name[int32(curState)], err)
				return
			}
			pbft.appendLogMsg(signedMsg)
			// 广播消息
			pbft.switcher.Broadcast(signedMsg)
			// 启动超时定时器器
			pbft.timer.Reset(10 * time.Second)
			// 直接迁移到 prepare状态
			pbft.sm.ChangeState(model.States_Preparing)

		} else {
			// 如果此次自己不是主验证者 切换到pre-prepare状态 开启超时 等待接收pre-prepare消息
			pbft.logger.Debugf("当前状态为 %s, 提议的新区块为: %d, 视图编号为: %d, 当前节点为副本验证节点 转换到pre-prepareing状态",
				model.States_name[int32(curState)], pbft.ws.BlockNum+1, pbft.ws.View)
			pbft.sm.ChangeState(model.States_PrePreparing)
			pbft.timer.Reset(10 * time.Second)
		}

	case model.States_PrePreparing:
		content := msg.GetGeneric()
		if content == nil {
			// pbft.appendLogMsg(msg)
			pbft.logger.Warnf("当前状态为%s 但是收到的消息类型为ViewChange", model.States_name[int32(curState)])
			return
		}
		msgType := content.Info.MsgType
		if msgType != model.MessageType_PrePrepare {
			// pbft.appendLogMsg(msg)
			pbft.logger.Warnf("当前节点处于PrePreparing 期待收到的消息类型为PrePrepare 此次消息类型为: %s, 忽略此次消息 ",
				model.MessageType_name[int32(msgType)])
			return
		}

		// 说明收到了 pre-prepare消息 验证消息内容是否是期望的 验证签名结果是否正确 验证是否是主节点签名
		if content.Info.SeqNum != pbft.ws.BlockNum+1 {
			// pbft.appendLogMsg(msg)
			pbft.logger.Warnf("当前节点处于PrePreparing 收到pre-prepare消息 但是区块编号和本地区块编号不一致, 收到的序号为: %d, 本地区块编号为: %d",
				content.Info.SeqNum, pbft.ws.BlockNum+1)
			return
		}
		if content.Info.View < pbft.ws.View {
			// pbft.appendLogMsg(msg)
			pbft.logger.Warnf("当前节点处于PrePreparing 收到pre-prepare消息 但是视图编号小于本地视图编号, 收到的视图序号为: %d, 本地视图序号为: %d",
				content.Info.View, pbft.ws.View)
			return
		}

		primary := (pbft.ws.BlockNum + 1 + content.Info.View) % uint64(len(pbft.ws.Verifiers))
		// 验证是否是主节点发送的
		if bytes.Compare(content.Info.SignerId, pbft.ws.Verifiers[primary].PublickKey) != 0 {
			pbft.logger.Warnf("当前节点处于PrePreparing 收到pre-prepare消息 计算的主节点的公钥不一致, 主节点标号为: %d",
				primary)
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
		}

		// 签名
		signedInfo, err := pbft.signMsgInfo(newMsg.Info)
		if err != nil {
			pbft.logger.Debugf("当前状态为 %s, 发起prepare消息时 在签名过程中发生错误 err: %v ",
				model.States_name[int32(curState)], err)
			return
		}
		newMsg.Info = signedInfo
		signedMsg := model.NewPbftMessage(newMsg)
		pbft.appendLogMsg(signedMsg)

		// 广播消息
		pbft.switcher.Broadcast(signedMsg)
		pbft.sm.ChangeState(model.States_Preparing)
		pbft.timer.Reset(10 * time.Second)

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
		// isVerifier := false
		// for i := range pbft.ws.Verifiers {
		// 	if bytes.Compare(content.Info.SignerId, pbft.ws.Verifiers[i].PublickKey) == 0 {
		// 		isVerifier = true
		// 		break
		// 	}
		// }
		// if !isVerifier {
		// 	pbft.logger.Warnf("当前节点处于Preparing 收到prepare消息 但是此验证者公钥不在验证列表中, 公钥内容: %s",
		// 		string(content.Info.SignerId))
		// 	return
		// }

		// 加入到prepare列表 当满足大于 2f+1时 进入checking状态
		// for _, m := range content.OtherInfos {
		// 	pbft.appendLogMsg(model.NewPbftMessage(&model.PbftGenericMessage{Info: m}))
		// }
		// 计算fault 数量
		f := (len(pbft.ws.Verifiers) - 1) / 3
		minNodes := 2*f + 1
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
				}

				nodes[string(log.msg.GetGeneric().Info.SignerId)] = true
			}
			if len(nodes) >= minNodes {
				// 满足节点数量  进入checking
				pbft.sm.ChangeState(model.States_Checking)
				// todo:: 如果本机已经保存了完整区块 广播区块
				pbft.timer.Reset(10 * time.Second)
				// 触发信号 迁移到checking状态
				// 为啥需要手动触发 是由于 可能这个消息已经被收到了 到这一直不会收到消息转移到checking状态
				pbft.tiggerMigrate(model.States_Checking)
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
		pbft.switcher.Broadcast(model.NewPbftMessage(newMsg))

	case model.States_Checking:
		// // 先看看本地是否已经有有效的区块包
		// logMsg := pbft.sm.logMsg[pbft.ws.BlockNum+1]
		// for i := range logMsg {
		// 	if logMsg[i].block != nil && logMsg[i].block.BlockNum == pbft.ws.BlockNum+1 {
		// 		return
		// 	}
		// }
		// 说明节点已经收到了足够多的prepare消息 等待收到 完整的区块包
		content := msg.GetGeneric()
		if content == nil {
			pbft.logger.Warnf("当前状态为%s 但是收到的消息类型为ViewChange", model.States_name[int32(curState)])
			return
		}
		// 检查消息体中是否有有效的区块
		if content.Block == nil {
			return
		}
		if content.Block.BlockNum != pbft.ws.BlockNum+1 {
			return
		}
		primary := (pbft.ws.BlockNum + 1 + content.Info.View) % uint64(len(pbft.ws.Verifiers))
		// 验证是否是主节点发送的
		if bytes.Compare(content.Block.SignerId, pbft.ws.Verifiers[primary].PublickKey) != 0 {
			return
		}
		// 说明已经收到提交的区块 并且验证通过
		// 广播Commit消息
		newMsg := model.PbftGenericMessage{
			Info: &model.PbftMessageInfo{MsgType: model.MessageType_Commit,
				View: pbft.ws.View, SeqNum: pbft.ws.BlockNum + 1,
				SignerId: pbft.ws.CurVerfier.PublickKey,
				Sign:     nil,
			},
			Block: content.Block,
		}
		signedInfo, err := pbft.signMsgInfo(newMsg.Info)
		if err != nil {
			pbft.logger.Debugf("当前状态为 %s, 发起commit消息时 在签名过程中发生错误 err: %v ",
				model.States_name[int32(curState)], err)
			return
		}
		newMsg.Info = signedInfo

		pbft.appendLogMsg(model.NewPbftMessage(newMsg))
		pbft.switcher.Broadcast(model.NewPbftMessage(newMsg))
		pbft.sm.ChangeState(model.States_Committing)
		pbft.timer.Reset(10 * time.Second)

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
		f := (len(pbft.ws.Verifiers) - 1) / 3
		minNodes := 2*f + 1
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
				}
				nodes[string(log.msg.GetGeneric().Info.SignerId)] = true
			}
			if len(nodes) >= minNodes {
				// 满足节点数量  进入checking
				pbft.sm.ChangeState(model.States_Finished)
				pbft.timer.Reset(10 * time.Second)
				// 触发信号 迁移到checking状态
				// 为啥需要手动触发 是由于 可能这个消息已经被收到了 到这一直不会收到消息转移到checking状态
				pbft.tiggerMigrate(model.States_Finished)
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
		pbft.switcher.Broadcast(model.NewPbftMessage(newMsg))
	}
}

func (pbft *PBFT) tiggerMigrateProcess(s model.States) {
	pbft.tiggerTimer.Stop()
	defer func() { pbft.tiggerTimer.Reset(300 * time.Millisecond) }()

	curState := pbft.sm.CurrentState()
	switch curState {
	case model.States_NotStartd:
		// 暂时啥也不做
	case model.States_PrePreparing:
		logMsg := pbft.sm.logMsg[pbft.ws.BlockNum+1]
		primary := (pbft.ws.BlockNum + 1 + pbft.ws.View) % uint64(len(pbft.ws.Verifiers))
		for i := range logMsg {
			if logMsg[i].MessageType != model.MessageType_PrePrepare {
				continue
			}
			// 验证是否是主节点发送的
			if bytes.Compare(logMsg[i].msg.GetGeneric().Info.SignerId, pbft.ws.Verifiers[primary].PublickKey) != 0 {
				continue
			}
			if logMsg[i].view != pbft.ws.View {
				continue
			}

			pbft.sm.ChangeState(model.States_Preparing)
			pbft.timer.Reset(10 * time.Second)
			break
		}

	case model.States_Preparing:
		logMsg := pbft.sm.logMsg[pbft.ws.BlockNum+1]
		// 计算fault 数量
		f := (len(pbft.ws.Verifiers) - 1) / 3
		minNodes := 2*f + 1
		nodes := make(map[string]bool)
		for _, log := range logMsg {
			if log.MessageType == model.MessageType_Prepare && log.view == pbft.ws.View {
				nodes[string(log.msg.GetGeneric().Info.SignerId)] = true
			}
			if len(nodes) >= minNodes {
				// 满足节点数量  进入checking
				pbft.sm.ChangeState(model.States_Checking)
				pbft.timer.Reset(10 * time.Second)
				break
			}
		}

	case model.States_Checking:
		// 本地是否已经有有效的区块包
		logMsg := pbft.sm.logMsg[pbft.ws.BlockNum+1]
		for i := range logMsg {
			if logMsg[i].block != nil && logMsg[i].block.BlockNum == pbft.ws.BlockNum+1 {
				primary := (pbft.ws.BlockNum + 1 + pbft.ws.View) % uint64(len(pbft.ws.Verifiers))
				// 验证是否是主节点发送的
				if bytes.Compare(logMsg[i].block.SignerId, pbft.ws.Verifiers[primary].PublickKey) != 0 {
					continue
				}
				pbft.sm.ChangeState(model.States_Committing)
				pbft.timer.Reset(10 * time.Second)
				break
			}
		}

	case model.States_Committing:
		logMsg := pbft.sm.logMsg[pbft.ws.BlockNum+1]
		// 计算fault 数量
		f := (len(pbft.ws.Verifiers) - 1) / 3
		minNodes := 2*f + 1
		nodes := make(map[string]bool)
		for _, log := range logMsg {
			if log.MessageType == model.MessageType_Commit && log.view == pbft.ws.View {
				nodes[string(log.msg.GetGeneric().Info.SignerId)] = true
			}
			if len(nodes) >= minNodes {
				// 满足节点数量  进入finisd
				pbft.sm.ChangeState(model.States_Finished)
				pbft.timer.Reset(10 * time.Second)
				return
			}
		}
	}
}
