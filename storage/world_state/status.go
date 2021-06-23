package world_state

import (
	"sync"

	"github.com/wupeaking/pbft_impl/model"
	"github.com/wupeaking/pbft_impl/storage/cache"
)

// 定义整个全局状态

type WroldState struct {
	BlockNum     uint64            `json:"blockNum"`  // 当前区块编号
	PrevBlock    string            `json:"prevBlock"` // 前一个区块hash
	BlockID      string            `json:"blockID"`   // 当前区块hash
	Verifiers    []*model.Verifier `json:"verifiers"` // 当前所有的验证者
	VerifiersMap map[string]struct{}
	VerifierNo   int             `josn:"verifierNo"` // 验证者所处编号 如果为-1  表示不是验证者
	CurVerfier   *model.Verifier `json:"curVerfier"`
	View         uint64          `json:"view"` // 当前视图
	db           *cache.DBCache
	sync.RWMutex `json:"-"`
}

func New(dbCache *cache.DBCache) *WroldState {
	return &WroldState{db: dbCache}
}

func (ws *WroldState) IncreaseBlockNum() {
	ws.Lock()
	ws.BlockNum++
	ws.Unlock()
}

func (ws *WroldState) IncreaseView() {
	ws.Lock()
	ws.View++
	ws.Unlock()
}

func (ws *WroldState) SetBlockNum(num uint64) {
	ws.Lock()
	defer func() { ws.Unlock() }()
	ws.BlockNum = num
}

func (ws *WroldState) SetView(v uint64) {
	ws.Lock()
	defer func() { ws.Unlock() }()
	ws.View = v
}

func (ws *WroldState) SetValue(blockNum uint64, prevBlock string, blockID string,
	verifiers []*model.Verifier) {
	ws.Lock()
	defer func() { ws.Unlock() }()
	if blockNum != 0 {
		ws.BlockNum = blockNum
	}
	if prevBlock != "" {
		ws.PrevBlock = prevBlock
	}
	if blockID != "" {
		ws.BlockID = blockID
	}
	if len(verifiers) > 0 {
		ws.Verifiers = verifiers
	}
}

func (ws *WroldState) IsVerfier(publicKey []byte) bool {
	ws.RLock()
	defer func() { ws.RUnlock() }()
	_, ok := ws.VerifiersMap[string(publicKey)]
	return ok
}

func (ws *WroldState) updateVerifierMap() {
	ws.Lock()
	defer func() { ws.Unlock() }()
	newValue := make(map[string]struct{})
	for i := range ws.Verifiers {
		newValue[string(ws.Verifiers[i].PublickKey)] = struct{}{}
	}
	ws.VerifiersMap = newValue
}
