package consensus

import (
	"sync"
	"time"

	"github.com/emirpasic/gods/lists/singlylinkedlist"
	log "github.com/sirupsen/logrus"
	"github.com/wupeaking/pbft_impl/model"
	"github.com/wupeaking/pbft_impl/network"
	"github.com/wupeaking/pbft_impl/storage/world_state"
)

type PBFT struct {
	// 当前所属状态
	sm          *StateMachine
	verifiers   map[string]model.Verifier
	Msgs        *MsgQueue
	timer       *time.Timer
	switcher    network.SwitcherI
	logger      *log.Logger
	ws          *world_state.WroldState
	stateMigSig chan model.States // 状态迁移信号
}

type MsgQueue struct {
	l          *singlylinkedlist.List
	comingFlag chan struct{}
	size       int
	sync.Mutex
}

func NewMsgQueue() *MsgQueue {
	return &MsgQueue{
		l:          singlylinkedlist.New(),
		comingFlag: make(chan struct{}, 1000),
		size:       1000,
	}
}

func (mq *MsgQueue) InsertMsg(msg *model.PbftMessage) {
	mq.l.Add(msg)

	select {
	case mq.comingFlag <- struct{}{}:
		return
	default:

	}
}

func (mq *MsgQueue) GetMsg() *model.PbftMessage {
	v, ok := mq.l.Get(0)
	if !ok {
		return nil
	}
	return v.(*model.PbftMessage)
}

func (mq *MsgQueue) WaitMsg() <-chan struct{} {
	return mq.comingFlag
}

func New(ws *world_state.WroldState) (*PBFT, error) {
	pbft := &PBFT{}
	pbft.Msgs = NewMsgQueue()
	pbft.sm = NewStateMachine()
	pbft.timer = time.NewTimer(10 * time.Second)
	pbft.timer.Stop()
	pbft.logger = log.New()
	pbft.logger.SetLevel(log.DebugLevel)
	pbft.logger.WithField("module", "consensus")
	pbft.ws = ws
	pbft.verifiers = make(map[string]model.Verifier)
	for _, v := range ws.Verifiers {
		pbft.verifiers[string(v.PublickKey)] = v
	}
	pbft.stateMigSig = make(chan model.States, 1)

	return pbft, nil
}

func (pbft *PBFT) Daemon() {
	// 启动超时定时器
	pbft.timer.Reset(10 * time.Second)
	for {
		select {
		case <-pbft.Msgs.WaitMsg():
			// 有消息进入
			pbft.StateMigrate(pbft.Msgs.GetMsg())

			// switch pbft.sm.CurrentState() {
			// case model.States_NotStartd:

			// case model.States_PrePreparing:
			// case model.States_Preparing:
			// case model.States_Checking:
			// case model.States_Committing:
			// case model.States_Finished:
			// case model.States_ViewChanging:
			// }

		case <-pbft.timer.C:
			// 有超时
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
