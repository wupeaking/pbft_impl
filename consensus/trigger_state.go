package consensus

import (
	"bytes"
	"time"

	"github.com/wupeaking/pbft_impl/model"
)

func (pbft *PBFT) tiggerMigrateProcess(s model.States) {
	//pbft.logger.Debugf("进入触发迁移处理函数, 当前状态: %v", pbft.sm.state)
	//pbft.tiggerTimer.Stop()
	// defer func() { pbft.tiggerTimer.Reset(1000 * time.Millisecond) }()

	curState := pbft.sm.CurrentState()
	switch curState {
	case model.States_NotStartd:
		// 暂时啥也不做
	case model.States_PrePreparing:
		logMsg := pbft.sm.logMsg[pbft.ws.BlockNum+1]
		for i := range logMsg {
			if logMsg[i].MessageType != model.MessageType_PrePrepare {
				continue
			}
			// 验证是否是主节点发送的
			if !pbft.IsPrimaryVerfier() {
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
		f := len(pbft.ws.Verifiers) / 3
		var minNodes int
		if f == 0 {
			minNodes = len(pbft.ws.Verifiers)
		} else {
			minNodes = 2*f + 1
		}
		nodes := make(map[string]bool)
		for _, log := range logMsg {
			if log.MessageType == model.MessageType_Prepare && log.view == pbft.ws.View {
				if bytes.Compare(log.msg.GetGeneric().Info.SignerId, pbft.ws.CurVerfier.PublickKey) == 0 {
					continue
				}
				nodes[string(log.msg.GetGeneric().Info.SignerId)] = true
			}
		}
		if len(nodes) >= minNodes-1 {
			// 满足节点数量  进入checking
			pbft.sm.ChangeState(model.States_Checking)
			pbft.timer.Reset(10 * time.Second)
			break
		}

	case model.States_Checking:
		// 本地是否已经有有效的区块包
		logMsg := pbft.sm.logMsg[pbft.ws.BlockNum+1]
		for i := range logMsg {
			if logMsg[i].block != nil && logMsg[i].block.BlockNum == pbft.ws.BlockNum+1 {
				// 验证是否是主节点发送的
				if !pbft.IsPrimaryVerfier() {
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
		f := len(pbft.ws.Verifiers) / 3
		var minNodes int
		if f == 0 {
			minNodes = len(pbft.ws.Verifiers)
		} else {
			minNodes = 2*f + 1
		}
		nodes := make(map[string]bool)
		for _, log := range logMsg {
			if log.MessageType == model.MessageType_Commit && log.view == pbft.ws.View {
				if bytes.Compare(log.msg.GetGeneric().Info.SignerId, pbft.ws.CurVerfier.PublickKey) == 0 {
					continue
				}
				nodes[string(log.msg.GetGeneric().Info.SignerId)] = true
			}
		}
		if len(nodes) >= minNodes-1 {
			// 满足节点数量  进入finisd
			pbft.sm.ChangeState(model.States_Finished)
			pbft.timer.Reset(10 * time.Second)
			return
		}
	case model.States_Finished:
		// play block
		// 更新到最新的状态
		// 切换到not start  等待下一轮循环
		pbft.timer.Stop()
		pbft.CommitBlock(pbft.sm.receivedBlock)
		pbft.sm.ChangeState(model.States_NotStartd)

	case model.States_ViewChanging:
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
				}
				nodes[string(log.msg.GetViewChange().Info.SignerId)] = true
			}
			if len(nodes) >= minNodes {
				// 满足节点数量   进入not start view +1
				pbft.ws.IncreaseView()
				pbft.sm.ChangeState(model.States_NotStartd)
				// pbft.timer.Reset(10 * time.Second)
				return
			}
		}
	}
}
