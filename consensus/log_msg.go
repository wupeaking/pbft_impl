package consensus

import (
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
	BlockMsg  map[string]*model.PbftBlock
	StateMsgs map[string][]*StateMsg
	BlockView map[uint64][]int
}

func NewMsgManager() *MsgManager {
	return &MsgManager{
		BlockMsg:  make(map[string]*model.PbftBlock),
		StateMsgs: make(map[string][]*StateMsg),
		BlockView: make(map[uint64][]int),
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

func (mm *MsgManager) addMsg(blkNum uint64, view int, msgType model.MessageType,
	signer []byte, msg *model.PbftMessage, gm *model.PbftGenericMessage, vc *model.PbftViewChange) bool {
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
		logMsgs := pbft.sm.logMsg[content.Info.SeqNum]
		if logMsgs == nil {
			logMsgs = make(LogGroupByType)
		}
		typeView := fmt.Sprintf("%d-%d", content.Info.MsgType, content.Info.View)
		msgBySinger := logMsgs[typeView]
		if msgBySinger == nil {
			msgBySinger = make(LogGroupBySigner)
		}
		msgBySinger[string(signer)] = LogMessage{
			MessageType: content.Info.MsgType,
			msg:         msg,
			view:        content.Info.View,
		}
		logMsgs[typeView] = msgBySinger

		if content.Block != nil {
			logBlks := pbft.sm.logBlock[content.Info.SeqNum]
			if logBlks == nil {
				logBlks = make(map[int]*model.PbftBlock)
			}
			logBlks[len(content.Block.SignPairs)] = content.Block
			pbft.sm.logBlock[content.Info.SeqNum] = logBlks
		}

		// pbft.logger.Debugf("追加日志高度: %d, 日志类型: %s", content.Info.SeqNum, content.Info.GetMsgType())
		pbft.sm.logMsg[content.Info.SeqNum] = logMsgs
		return true
	case *model.PbftViewChange:
		logMsgs := pbft.sm.logMsg[content.Info.SeqNum]
		if logMsgs == nil {
			logMsgs = make(LogGroupByType)
		}
		typeView := fmt.Sprintf("%d-%d", content.Info.MsgType, content.Info.View)
		msgBySinger := logMsgs[typeView]
		if msgBySinger == nil {
			msgBySinger = make(LogGroupBySigner)
		}
		msgBySinger[string(signer)] = LogMessage{
			MessageType: content.Info.MsgType,
			msg:         msg,
			view:        content.Info.View,
		}
		logMsgs[typeView] = msgBySinger
		pbft.sm.logMsg[content.Info.SeqNum] = logMsgs
	}
	return false
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
func (pbft *PBFT) appendLogMsg(msg *model.PbftMessage) bool {
	content := msg.GetGeneric()
	signer := pbft.GetMsgSigner(msg)
	if signer == nil {
		return false
	}
	if content != nil {
		for i := range content.OtherInfos {
			pbft.appendLogMsg(model.NewPbftMessage(&model.PbftGenericMessage{Info: content.OtherInfos[i]}))
		}
		logMsgs := pbft.sm.logMsg[content.Info.SeqNum]
		if logMsgs == nil {
			logMsgs = make(LogGroupByType)
		}
		typeView := fmt.Sprintf("%d-%d", content.Info.MsgType, content.Info.View)
		msgBySinger := logMsgs[typeView]
		if msgBySinger == nil {
			msgBySinger = make(LogGroupBySigner)
		}
		// if _, ok := msgBySinger[string(signer)]; ok {
		// 	// 如果已经记录 就不在记录
		// 	return false
		// }
		msgBySinger[string(signer)] = LogMessage{
			MessageType: content.Info.MsgType,
			msg:         msg,
			view:        content.Info.View,
			// block:       content.Block,
		}
		logMsgs[typeView] = msgBySinger

		if content.Block != nil {
			logBlks := pbft.sm.logBlock[content.Info.SeqNum]
			if logBlks == nil {
				logBlks = make(map[int]*model.PbftBlock)
			}
			logBlks[len(content.Block.SignPairs)] = content.Block
			pbft.sm.logBlock[content.Info.SeqNum] = logBlks
		}

		// pbft.logger.Debugf("追加日志高度: %d, 日志类型: %s", content.Info.SeqNum, content.Info.GetMsgType())
		pbft.sm.logMsg[content.Info.SeqNum] = logMsgs
		return true
	}
	if content := msg.GetViewChange(); content != nil {
		logMsgs := pbft.sm.logMsg[content.Info.SeqNum]
		if logMsgs == nil {
			logMsgs = make(LogGroupByType)
		}
		typeView := fmt.Sprintf("%d-%d", content.Info.MsgType, content.Info.View)
		msgBySinger := logMsgs[typeView]
		if msgBySinger == nil {
			msgBySinger = make(LogGroupBySigner)
		}
		msgBySinger[string(signer)] = LogMessage{
			MessageType: content.Info.MsgType,
			msg:         msg,
			view:        content.Info.View,
		}
		logMsgs[typeView] = msgBySinger
		pbft.sm.logMsg[content.Info.SeqNum] = logMsgs
	}
	return false
}

// 定时清除无用的logMsg
func (pbft *PBFT) garbageCollection() {
	for {
		select {
		case <-time.After(10 * time.Second):
			for key := range pbft.sm.logMsg {
				// 保留个阈值
				if key+10 < pbft.ws.BlockNum {
					pbft.logger.Debugf("删除key: %v", key)
					delete(pbft.sm.logMsg, key)
				}
			}
		}
	}
}
