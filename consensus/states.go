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
	msg  *model.PbftMessage
	view uint64
}

func (pbft *PBFT) appendLogMsg(msg *model.PbftMessage) {
	content := msg.GetGeneric()
	if content != nil {
		logMsgs := pbft.sm.logMsg[content.Info.SeqNum]
		logMsgs = append(logMsgs, LogMessage{
			MessageType: content.Info.MsgType,
			msg:         msg,
			view:        content.Info.View,
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
	// todo:: 校验消息的合法性 是否为空值 如果不合法 直接返回
	curState := pbft.sm.CurrentState()
	switch curState {
	case model.States_NotStartd:
		// 处于此状态 期望接收到 新区块提议
		content := msg.GetGeneric()
		if content == nil {
			pbft.appendLogMsg(msg)
			pbft.logger.Errorf("当前状态为%s 但是收到的消息类型为ViewChange", model.States_name[int32(curState)])
			return
		}

		msgType := content.Info.MsgType
		if msgType != model.MessageType_NewBlockProposal {
			pbft.appendLogMsg(msg)
			pbft.logger.Errorf("当前状态为%s 期待收到新区块提议 但是收到的消息类型为%s",
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
					Sign:     nil, // todo:: 需要签名
				},
			}
			// 广播消息
			pbft.switcher.Broadcast(model.NewPbftMessage(newMsg))
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
		// pbft.timer.Stop()
		// defer func() { pbft.timer.Reset(10 * time.Second) }()

		content := msg.GetGeneric()
		if content == nil {
			pbft.appendLogMsg(msg)
			pbft.logger.Errorf("当前状态为%s 但是收到的消息类型为ViewChange", model.States_name[int32(curState)])
			return
		}
		msgType := content.Info.MsgType
		if msgType != model.MessageType_PrePrepare {
			pbft.appendLogMsg(msg)
			pbft.logger.Warnf("当前节点处于PrePreparing 期待收到的消息类型为PrePrepare 此次消息类型为: %s, 忽略此次消息 ",
				model.MessageType_name[int32(msgType)])
			return
		}

		// 说明收到了 pre-prepare消息 验证消息内容是否是期望的 验证签名结果是否正确 验证是否是主节点签名
		if content.Info.SeqNum != pbft.ws.BlockNum+1 {
			pbft.appendLogMsg(msg)
			pbft.logger.Warnf("当前节点处于PrePreparing 收到pre-prepare消息 但是区块编号和本地区块编号不一致, 收到的序号为: %d, 本地区块编号为: %d",
				content.Info.SeqNum, pbft.ws.BlockNum+1)
			return
		}
		if content.Info.View < pbft.ws.View {
			pbft.appendLogMsg(msg)
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
		// todo :: 验证签名内容
		{
		}
		// 执行到此处 说明收到了正确的由主节点发送的pre-prepare消息
		// 广播prepare消息 切换到preparing状态  重置超时 等待接收足够多的prepare消息
		// 向所有验证者发起pre-prepare 消息
		newMsg := model.PbftGenericMessage{
			Info: &model.PbftMessageInfo{MsgType: model.MessageType_Prepare,
				View: pbft.ws.View, SeqNum: pbft.ws.BlockNum + 1,
				SignerId: pbft.ws.CurVerfier.PublickKey,
				Sign:     nil, // todo:: 需要签名
			},
		}
		pbft.appendLogMsg(model.NewPbftMessage(newMsg))
		// 广播消息
		pbft.switcher.Broadcast(model.NewPbftMessage(newMsg))
		pbft.sm.ChangeState(model.States_Preparing)
		pbft.timer.Reset(10 * time.Second)

	case model.States_Preparing:
		// 此状态需要接收足够多的prepare消息 方能迁移成功
		content := msg.GetGeneric()
		if content == nil {
			pbft.logger.Errorf("当前状态为%s 但是收到的消息类型为ViewChange", model.States_name[int32(curState)])
			return
		}
		msgType := content.Info.MsgType
		if msgType != model.MessageType_Prepare {
			pbft.appendLogMsg(msg)
			pbft.logger.Warnf("当前节点处于Preparing 期待收到的消息类型为Prepare 此次消息类型为: %s, 忽略此次消息 ",
				model.MessageType_name[int32(msgType)])
			return
		}
		// 1.需要区块编号和本机对应 2.视图编号与本机对应 3.验证者处于验证者列表中 4.签名正确
		if content.Info.SeqNum != pbft.ws.BlockNum+1 {
			pbft.appendLogMsg(msg)
			pbft.logger.Warnf("当前节点处于Preparing 收到prepare消息 但是区块编号和本地区块编号不一致, 收到的序号为: %d, 本地区块编号为: %d",
				content.Info.SeqNum, pbft.ws.BlockNum+1)
			return
		}
		if content.Info.View != pbft.ws.View {
			pbft.appendLogMsg(msg)
			pbft.logger.Warnf("当前节点处于Preparing 收到prepare消息 但是视图编号与本地视图编号不一致, 收到的视图序号为: %d, 本地视图序号为: %d",
				content.Info.View, pbft.ws.View)
			return
		}
		isVerifier := false
		for i := range pbft.ws.Verifiers {
			if bytes.Compare(content.Info.SignerId, pbft.ws.Verifiers[i].PublickKey) == 0 {
				isVerifier = true
				break
			}
		}
		if !isVerifier {
			pbft.logger.Warnf("当前节点处于Preparing 收到prepare消息 但是此验证者公钥不在验证列表中, 公钥内容: %s",
				string(content.Info.SignerId))
			return
		}
		// todo:: 待验证签名
		{
		}
		// 加入到prepare列表 当满足大于 2f+1时 进入checking状态
		for _, m := range content.OtherInfos {
			pbft.appendLogMsg(model.NewPbftMessage(&model.PbftGenericMessage{Info: m}))
		}
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
		} else {
			newMsg.Info = localprepareMsg
		}
		newMsg.OtherInfos = prepareMsgs
		// 广播消息
		pbft.switcher.Broadcast(model.NewPbftMessage(newMsg))
	}
}
