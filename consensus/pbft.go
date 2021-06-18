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

/*
问题:
1. 广播消息没有指定peer 任意广播 会出现消息爆炸传递
2. 定时广播出现过多消息传递 进入viewchange 广播消息类型不对 不定时广播如何解决对方没有收到的问题
3. 收到消息没有判断是否是重复 重复进入状态迁移

新架构方案:
1. 共识的状态的转移依旧以消息驱动为主, 但是不在频繁进行消息广播. 每次状态迁移只广播一次消息.
2. 为了解决因为消息驱动不及时导致的状态不能迁移问题. 加上定时轮询
3. 重构消息管理 包括消息存储 消息查找 消息删除 消息标记
4. 消息广播任务简单化 只有在viewchange状态 才持续广播
*/

type PBFT struct {
	// 当前所属状态
	sm               *StateMachine
	mm               *MsgManager
	verifiers        map[string]*model.Verifier
	verifierPeerID   map[string]string // peerID --- string(singer)
	Msgs             *MsgQueue
	stateTimeout     *time.Timer // 状态转换超时器
	switcher         network.SwitcherI
	logger           *log.Entry
	ws               *world_state.WroldState
	stateMigTimer    *time.Timer // 状态迁移轮询定时器
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
		comingMsg: make(chan *model.PbftMessage, 10000),
		size:      1000,
	}
}

func (mq *MsgQueue) InsertMsg(msg *model.PbftMessage) {
	select {
	case mq.comingMsg <- msg:
		return
	default:
		println("消息写入通道已满...")
	}
}

func (mq *MsgQueue) WaitMsg() <-chan *model.PbftMessage {
	return mq.comingMsg
}

func New(ws *world_state.WroldState, txPool *transaction.TxPool, switcher network.SwitcherI, cfg *config.Configure) (*PBFT, error) {
	pbft := &PBFT{}
	pbft.cfg = cfg
	pbft.Msgs = NewMsgQueue()
	pbft.sm = NewStateMachine()

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
	pbft.verifierPeerID = make(map[string]string)

	// 转换验证者的peer id
	if err := pbft.LoadVerfierPeerIDs(); err != nil {
		return nil, err
	}

	pbft.broadcastSig = make(chan *model.PbftMessage, 100)
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

	pbft.stateTimeout = time.NewTimer(10 * time.Second)
	pbft.tryProposalTimer = time.NewTimer(5 * time.Second)
	pbft.stateMigTimer = time.NewTimer(500 * time.Millisecond)

	for {
		select {
		case msg := <-pbft.Msgs.WaitMsg():
			if pbft.StopFlag {
				continue
			}
			// 有消息进入
			pbft.StateMigrate(msg)

		case <-pbft.stateMigTimer.C:
			if pbft.StopFlag {
				continue
			}
			// 定时轮询状态迁移
			pbft.StateMigrate(nil)

		case <-pbft.stateTimeout.C:
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
				continue
			}
			pbft.AddBroadcastTask(signedMsg)

		case <-pbft.tryProposalTimer.C:
			// 1. 检查共识引擎是否可以开始 2.是否处于no_started状态 3. 发起提案广播
			// 重置timer 重置需要先停止 停止的时候要检查是否已经过期 过期可能需要尝试清空通道
			if !pbft.tryProposalTimer.Stop() {
				select {
				case <-pbft.tryProposalTimer.C: // 要尝试抽空chanel的值
				default:
				}
			}
			pbft.tryProposalTimer.Reset(5 * time.Second)

			if pbft.StopFlag {
				continue
			}
			if pbft.CurrentState() != model.States_NotStartd {
				continue
			}
			pbft.logger.Debugf("尝试发起新提案...")
			pbft.requestNewBlockProposal()
		}

	}
}

// 注册到网络的消息回调
func (pbft *PBFT) msgOnRecv(modelID string, msgBytes []byte, p *network.Peer) {
	//pbft.logger.Debugf("收到其他节点发来的消息...")
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
	//pbft.logger.Debugf("收到其他节点发来的消息...")
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
