package consensus

import (
	"bytes"
	"fmt"
	"sync"
	"time"

	"github.com/wupeaking/pbft_impl/model"
)

/*
	block_num, view, msg_type, msg, signer(sender)

	block_num, view, blk
*/

type MsgManager struct {
	// block_num-view : blk
	BlockMsg      map[string]*model.PbftBlock
	BlockMsgLock  sync.RWMutex
	StateMsgs     map[string][]*StateMsg
	StateMsgLock  sync.RWMutex
	BlockView     map[uint64]map[uint64]struct{}
	BlockViewLock sync.RWMutex
}

func NewMsgManager() *MsgManager {
	return &MsgManager{
		BlockMsg:      make(map[string]*model.PbftBlock),
		BlockMsgLock:  sync.RWMutex{},
		StateMsgs:     make(map[string][]*StateMsg),
		StateMsgLock:  sync.RWMutex{},
		BlockView:     make(map[uint64]map[uint64]struct{}),
		BlockViewLock: sync.RWMutex{},
	}
}

type StateMsg struct {
	MsgType       model.MessageType
	Msg           *model.PbftMessage
	GenericMsg    *model.PbftGenericMessage
	ViewChangeMsg *model.PbftViewChange
	Signer        []byte
	Broadcast     bool // 是否被广播过
	Readed        bool // 是否在状态迁移阶段被读取过
	sync.RWMutex       //
}

func (mm *MsgManager) addMsg(blkNum uint64, view uint64, msgType model.MessageType,
	signer []byte, msg *model.PbftMessage, gm *model.PbftGenericMessage, vc *model.PbftViewChange) bool {
	stateMsgKey := fmt.Sprintf("%d-%d", blkNum, view)
	mm.StateMsgLock.Lock()
	defer mm.StateMsgLock.Unlock()

	msgs, ok := mm.StateMsgs[stateMsgKey]
	if !ok {
		stateMsg := StateMsg{
			MsgType:       msgType,
			Msg:           msg,
			GenericMsg:    gm,
			ViewChangeMsg: vc,
			Signer:        signer,
			Broadcast:     false,
			Readed:        false,
		}
		mm.StateMsgs[stateMsgKey] = []*StateMsg{&stateMsg}
		mm.addBlockView(blkNum, view)
		return true
	}

	exist := false
	for i := range msgs {
		if msgs[i].MsgType == msgType && bytes.Compare(msgs[i].Signer, signer) == 0 {
			exist = true
			break
		}
	}
	if exist {
		return false
	}
	msgs = append(msgs, &StateMsg{
		MsgType:       msgType,
		Msg:           msg,
		GenericMsg:    gm,
		ViewChangeMsg: vc,
		Signer:        signer,
		Broadcast:     false,
		Readed:        false,
	})
	mm.StateMsgs[stateMsgKey] = msgs
	mm.addBlockView(blkNum, view)
	return true
}

func (mm *MsgManager) addBlockView(blkNum, view uint64) {
	mm.BlockViewLock.Lock()
	defer mm.BlockViewLock.Unlock()
	views, ok := mm.BlockView[blkNum]
	if !ok {
		views := make(map[uint64]struct{})
		views[view] = struct{}{}
		mm.BlockView[blkNum] = views
		return
	}
	views[view] = struct{}{}
	mm.BlockView[blkNum] = views
	return
}

func (mm *MsgManager) addBlock(blkNum uint64, view uint64, block *model.PbftBlock) bool {
	blkMsgKey := fmt.Sprintf("%d-%d", blkNum, view)
	mm.BlockMsgLock.Lock()
	defer mm.BlockMsgLock.Unlock()

	blk, ok := mm.BlockMsg[blkMsgKey]
	if !ok {
		mm.BlockMsg[blkMsgKey] = blk
		mm.addBlockView(blkNum, view)
		return true
	}

	if len(blk.SignPairs) < len(block.SignPairs) {
		mm.BlockMsg[blkMsgKey] = block
		mm.addBlockView(blkNum, view)
		return true
	}
	return false
}

func (pbft *PBFT) AppendMsg(msg *model.PbftMessage) bool {
	signer := pbft.GetMsgSigner(msg)
	if signer == nil {
		return false
	}
	switch content := getPbftMsg(msg).(type) {
	case *model.PbftGenericMessage:
		for i := range content.OtherInfos {
			pbft.AppendMsg(model.NewPbftMessage(&model.PbftGenericMessage{Info: content.OtherInfos[i]}))
		}
		addMsgOk := pbft.mm.addMsg(content.Info.SeqNum, content.Info.View,
			content.Info.MsgType, content.Info.SignerId, msg, content, nil)

		if content.Block != nil {
			// 判断提议者签名是否正确
			primary := (pbft.ws.BlockNum + 1 + pbft.ws.View) % uint64(len(pbft.ws.Verifiers))
			if len(pbft.ws.Verifiers) == 1 {
				primary = 0
			}
			if bytes.Compare(pbft.ws.Verifiers[primary].PublickKey, content.Block.SignerId) != 0 {
				pbft.logger.Debugf("添加区块消息失败 因为当前区块的主签名不一致和计算的主签名不是同一个 blockNum: %d, view: %d, primary: %d",
					content.Info.SeqNum, content.Info.View, primary)
				return addMsgOk
			}
			addBlkOk := pbft.mm.addBlock(content.Info.SeqNum, content.Info.View, content.Block)

			// 添加消息和区块 有一个成功则任务添加成功 从而再次进入状态处理
			return addMsgOk || addBlkOk
		}

		pbft.logger.Debugf("追加日志高度: %d, 日志类型: %s", content.Info.SeqNum, content.Info.GetMsgType())
		return addMsgOk

	case *model.PbftViewChange:
		return pbft.mm.addMsg(content.Info.SeqNum, content.Info.View,
			content.Info.MsgType, content.Info.SignerId, msg, nil, content)
	}
	return false
}

func (pbft *PBFT) FindBlock(num, view uint64) *model.PbftBlock {
	pbft.mm.BlockMsgLock.RLock()
	defer pbft.mm.BlockMsgLock.RUnlock()
	return pbft.mm.BlockMsg[fmt.Sprintf("%d-%d", num, view)]
}

func (pbft *PBFT) FindStateMsg(num, view uint64, msgType model.MessageType) ([]*model.PbftGenericMessage, []*model.PbftViewChange) {
	pbft.mm.StateMsgLock.RLock()
	defer pbft.mm.StateMsgLock.RUnlock()
	msgs := pbft.mm.StateMsgs[fmt.Sprintf("%d-%d", num, view)]
	if msgType == model.MessageType_ViewChange {
		rets := make([]*model.PbftViewChange, 0)
		for i := range msgs {
			if msgs[i].MsgType == msgType {
				rets = append(rets, msgs[i].ViewChangeMsg)
			}
			return nil, rets
		}
	}

	rets := make([]*model.PbftGenericMessage, 0)
	for i := range msgs {
		if msgs[i].MsgType == msgType {
			rets = append(rets, msgs[i].GenericMsg)
		}
		return rets, nil
	}
	return nil, nil
}

type LogGroupByType map[string]LogGroupBySigner
type LogGroupBySigner map[string]LogMessage

type LogMessage struct {
	model.MessageType
	msg   *model.PbftMessage
	block *model.PbftBlock
	view  uint64
}

type LogMsgCollection map[uint64]LogGroupByType

// blknum: { singerpairs: block }
type LogBlockCollection map[uint64]map[int]*model.PbftBlock

func (lm LogMsgCollection) FindMsg(num uint64, msgType model.MessageType, view int) LogGroupBySigner {
	logByType, ok := lm[num]
	if !ok {
		return nil
	}
	return logByType[fmt.Sprintf("%d-%d", msgType, view)]
}

// 检查被signer签名的消息是否已经收到
func (lm LogMsgCollection) ExistMsgBySinger(num uint64, msgType model.MessageType, view int, signer string) bool {
	logByType, ok := lm[num]
	if !ok {
		return false
	}
	LogGroupBySigner, ok := logByType[fmt.Sprintf("%d-%d", msgType, view)]
	if !ok {
		return false
	}
	_, ok = LogGroupBySigner[signer]
	return ok
}

func (lm LogBlockCollection) FindBlock(num uint64) map[int]*model.PbftBlock {
	logBlks, ok := lm[num]
	if !ok {
		return nil
	}
	return logBlks
}

func (lm LogBlockCollection) ResetBlock(num uint64) {
	lm[num] = nil
}

// false 表示已近存在
// func (pbft *PBFT) appendLogMsg(msg *model.PbftMessage) bool {
// 	content := msg.GetGeneric()
// 	signer := pbft.GetMsgSigner(msg)
// 	if signer == nil {
// 		return false
// 	}
// 	if content != nil {
// 		for i := range content.OtherInfos {
// 			pbft.appendLogMsg(model.NewPbftMessage(&model.PbftGenericMessage{Info: content.OtherInfos[i]}))
// 		}
// 		logMsgs := pbft.sm.logMsg[content.Info.SeqNum]
// 		if logMsgs == nil {
// 			logMsgs = make(LogGroupByType)
// 		}
// 		typeView := fmt.Sprintf("%d-%d", content.Info.MsgType, content.Info.View)
// 		msgBySinger := logMsgs[typeView]
// 		if msgBySinger == nil {
// 			msgBySinger = make(LogGroupBySigner)
// 		}
// 		// if _, ok := msgBySinger[string(signer)]; ok {
// 		// 	// 如果已经记录 就不在记录
// 		// 	return false
// 		// }
// 		msgBySinger[string(signer)] = LogMessage{
// 			MessageType: content.Info.MsgType,
// 			msg:         msg,
// 			view:        content.Info.View,
// 			// block:       content.Block,
// 		}
// 		logMsgs[typeView] = msgBySinger

// 		if content.Block != nil {
// 			logBlks := pbft.sm.logBlock[content.Info.SeqNum]
// 			if logBlks == nil {
// 				logBlks = make(map[int]*model.PbftBlock)
// 			}
// 			logBlks[len(content.Block.SignPairs)] = content.Block
// 			pbft.sm.logBlock[content.Info.SeqNum] = logBlks
// 		}

// 		// pbft.logger.Debugf("追加日志高度: %d, 日志类型: %s", content.Info.SeqNum, content.Info.GetMsgType())
// 		pbft.sm.logMsg[content.Info.SeqNum] = logMsgs
// 		return true
// 	}
// 	if content := msg.GetViewChange(); content != nil {
// 		logMsgs := pbft.sm.logMsg[content.Info.SeqNum]
// 		if logMsgs == nil {
// 			logMsgs = make(LogGroupByType)
// 		}
// 		typeView := fmt.Sprintf("%d-%d", content.Info.MsgType, content.Info.View)
// 		msgBySinger := logMsgs[typeView]
// 		if msgBySinger == nil {
// 			msgBySinger = make(LogGroupBySigner)
// 		}
// 		msgBySinger[string(signer)] = LogMessage{
// 			MessageType: content.Info.MsgType,
// 			msg:         msg,
// 			view:        content.Info.View,
// 		}
// 		logMsgs[typeView] = msgBySinger
// 		pbft.sm.logMsg[content.Info.SeqNum] = logMsgs
// 	}
// 	return false
// }

// 定时清除无用的logMsg
func (pbft *PBFT) garbageCollection() {
	for {
		select {
		case <-time.After(10 * time.Second):
			for key := range pbft.mm.BlockView {
				// 保留个阈值
				if key+10 > pbft.ws.BlockNum {
					break
				}
				// 找到当前num下所有的view
				views := pbft.mm.BlockView[key]
				for view := range views {
					k := fmt.Sprintf("%d-%d", key, view)
					pbft.logger.Debugf("删除key: %s", k)
					delete(pbft.mm.StateMsgs, k)
					delete(pbft.mm.BlockMsg, k)
				}
				delete(pbft.mm.BlockView, key)
			}
		}
	}
}
