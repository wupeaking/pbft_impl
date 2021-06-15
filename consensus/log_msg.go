package consensus

import (
	"fmt"

	"github.com/wupeaking/pbft_impl/model"
)

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

		pbft.logger.Debugf("追加日志高度: %d, 日志类型: %s", content.Info.SeqNum, content.Info.GetMsgType())
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
