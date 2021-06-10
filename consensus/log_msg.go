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

func (lm LogMsgCollection) FindMsg(num uint64, msgType model.MessageType, view int) LogGroupBySigner {
	logByType, ok := lm[num]
	if !ok {
		return nil
	}
	return logByType[fmt.Sprintf("%d-%d", msgType, view)]
}

func (pbft *PBFT) appendLogMsg(msg *model.PbftMessage) {
	content := msg.GetGeneric()
	signer := pbft.GetMsgSigner(msg)
	if signer == nil {
		return
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
		msgBySinger[string(signer)] = LogMessage{
			MessageType: content.Info.MsgType,
			msg:         msg,
			view:        content.Info.View,
			block:       content.Block,
		}
		logMsgs[typeView] = msgBySinger
		pbft.logger.Warnf("追加日志高度: %d, 日志类型: %s", content.Info.SeqNum, content.Info.GetMsgType())
		pbft.sm.logMsg[content.Info.SeqNum] = logMsgs
		return
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
}
