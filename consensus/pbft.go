package consensus

import (
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/emirpasic/gods/lists/singlylinkedlist"
	"github.com/golang/protobuf/proto"
	log "github.com/sirupsen/logrus"
	"github.com/wupeaking/pbft_impl/common/config"
	"github.com/wupeaking/pbft_impl/model"
	"github.com/wupeaking/pbft_impl/network"
	"github.com/wupeaking/pbft_impl/storage/world_state"
	"github.com/wupeaking/pbft_impl/transaction"
)

type PBFT struct {
	// 当前所属状态
	sm               *StateMachine
	verifiers        map[string]*model.Verifier
	Msgs             *MsgQueue
	timer            *time.Timer // 状态转换超时器
	switcher         network.SwitcherI
	logger           *log.Entry
	ws               *world_state.WroldState
	stateMigSig      chan model.States // 状态迁移信号
	txPool           *transaction.TxPool
	tryProposalTimer *time.Timer // 定时尝试提议区块
	StopFlag         bool
	cfg              *config.Configure
	curBroadcastMsg  *model.PbftMessage
	broadcastSig     chan *model.PbftMessage
	sync.Mutex
}

type MsgQueue struct {
	l         *singlylinkedlist.List
	comingMsg chan *model.PbftMessage
	size      int
	sync.Mutex
}

func NewMsgQueue() *MsgQueue {
	return &MsgQueue{
		l:         singlylinkedlist.New(),
		comingMsg: make(chan *model.PbftMessage, 1000),
		size:      1000,
	}
}

func (mq *MsgQueue) InsertMsg(msg *model.PbftMessage) {
	select {
	case mq.comingMsg <- msg:
		return
	default:

	}
}

// func (mq *MsgQueue) GetMsg() *model.PbftMessage {
// 	v, ok := mq.l.Get(0)
// 	if !ok {
// 		return nil
// 	}
// 	mq.l.Remove(0)
// 	return v.(*model.PbftMessage)
// }

func (mq *MsgQueue) WaitMsg() <-chan *model.PbftMessage {
	return mq.comingMsg
}

func New(ws *world_state.WroldState, txPool *transaction.TxPool, switcher network.SwitcherI, cfg *config.Configure) (*PBFT, error) {
	pbft := &PBFT{}
	pbft.cfg = cfg
	pbft.Msgs = NewMsgQueue()
	pbft.sm = NewStateMachine()
	pbft.timer = time.NewTimer(10 * time.Second)
	pbft.timer.Stop()

	pbft.tryProposalTimer = time.NewTimer(5 * time.Second)
	pbft.tryProposalTimer.Stop()

	l := log.New()
	l.SetReportCaller(true)
	l.SetFormatter(&log.TextFormatter{
		DisableColors: true,
		FullTimestamp: true,
	})
	switch strings.ToLower(cfg.ConsensusCfg.LogLevel) {
	case "debug":
		l.SetLevel(log.DebugLevel)
	case "warn":
		l.SetLevel(log.WarnLevel)
	case "info":
		l.SetLevel(log.InfoLevel)
	case "error":
		l.SetLevel(log.ErrorLevel)
	default:
		l.SetLevel(log.InfoLevel)
	}
	pbft.logger = l.WithField("module", "consensus")

	pbft.ws = ws
	pbft.verifiers = make(map[string]*model.Verifier)
	for _, v := range ws.Verifiers {
		pbft.verifiers[string(v.PublickKey)] = v
	}
	pbft.stateMigSig = make(chan model.States, 1)
	pbft.broadcastSig = make(chan *model.PbftMessage, 1)
	pbft.txPool = txPool

	pbft.switcher = switcher
	pbft.StopFlag = true

	return pbft, nil
}

func (pbft *PBFT) Daemon() {
	// 注册消息回调
	pbft.switcher.RegisterOnReceive("consensus", pbft.msgOnRecv)

	go pbft.garbageCollection()
	go pbft.BroadcastMsgRoutine()

	// 启动定时提案
	pbft.tryProposalTimer.Reset(5 * time.Second)

	for {
		select {
		case msg := <-pbft.Msgs.WaitMsg():
			if pbft.StopFlag {
				continue
			}
			// 有消息进入
			pbft.StateMigrate(msg)

		// case s := <-pbft.sm.changeSig:
		// 	if pbft.StopFlag {
		// 		continue
		// 	}
		// 	pbft.tiggerMigrateProcess(s)

		case <-pbft.timer.C:
			if pbft.StopFlag {
				continue
			}
			// 有超时 则进入viewchang状态 发起viewchange消息
			pbft.logger.Debugf("超时 进入ViewChanging状态")
			pbft.ChangeState(model.States_ViewChanging)
			newMsg := model.PbftViewChange{
				Info: &model.PbftMessageInfo{MsgType: model.MessageType_ViewChange,
					View: pbft.ws.View, SeqNum: pbft.ws.BlockNum + 1,
					SignerId: pbft.ws.CurVerfier.PublickKey,
					Sign:     nil,
				},
			}
			signedMsg, err := pbft.SignMsg(model.NewPbftMessage(&newMsg))
			if err != nil {
				pbft.logger.Errorf("在viewchanging状态 进行消息签名时 发生了错误, err: %v", err)
				return
			}
			pbft.appendLogMsg(signedMsg)
			// pbft.broadcastStateMsg(signedMsg)
			pbft.AddBroadcastTask(signedMsg)
		case <-pbft.tryProposalTimer.C:
			// 1. 检查共识引擎是否可以开始 2.是否处于no_started状态 3. 发起提案广播
			pbft.tryProposalTimer.Reset(5 * time.Second)
			if pbft.StopFlag {
				return
			}
			if pbft.CurrentState() != model.States_NotStartd {
				return
			}
			pbft.requestNewBlockProposal()
		}

	}
}

func (pbft *PBFT) tiggerMigrate(s model.States) {
	select {
	case pbft.stateMigSig <- s:
		return
	default:
		return
	}
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

// 注册到网络的消息回调
func (pbft *PBFT) msgOnRecv(modelID string, msgBytes []byte, p *network.Peer) {
	if modelID != "consensus" {
		return
	}
	var msgPkg network.BroadcastMsg
	if json.Unmarshal(msgBytes, &msgPkg) != nil {
		pbft.logger.Debugf("共识模块收到网络包不能解析")
		return
	}
	var pbftMsg model.PbftMessage
	if proto.Unmarshal(msgPkg.Msg, &pbftMsg) != nil {
		pbft.logger.Debugf("共识模块收到消息不能解析")
		return
	}
	// if gm := pbftMsg.GetGeneric(); gm != nil {
	// 	pbft.logger.Infof("插入Msg的高度: %d", gm.Info.SeqNum)
	// }

	pbft.Msgs.InsertMsg(&pbftMsg)
}

func (pbft *PBFT) Start() {
	pbft.Lock()
	pbft.StopFlag = false
	pbft.Unlock()
}

func (pbft *PBFT) Stop() {
	pbft.Lock()
	pbft.StopFlag = true
	pbft.ChangeState(model.States_NotStartd)
	pbft.Unlock()
}

func (pbft *PBFT) requestNewBlockProposal() {
	msgInfo := model.PbftGenericMessage{
		Info: &model.PbftMessageInfo{
			MsgType: model.MessageType_NewBlockProposal,
			SeqNum:  pbft.ws.BlockNum + 1,
			View:    pbft.ws.View,
		},
	}
	// 签名
	signedMsg, err := pbft.SignMsg(model.NewPbftMessage(&msgInfo))
	if err != nil {
		pbft.logger.Debugf("发起新提案区块消息时 在签名过程中发生错误 err: %v ",
			err)
		return
	}
	pbft.Msgs.InsertMsg(signedMsg)
	// 广播消息
	pbft.broadcastStateMsg(signedMsg)
}
