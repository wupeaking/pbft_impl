package world_state

import "github.com/wupeaking/pbft_impl/model"

// 定义整个全局状态

type WroldState struct {
	BlockNum   uint64           // 当前区块编号
	PrevBlock  string           // 前一个区块hash
	BlockID    string           // 当前区块hash
	Verifiers  []model.Verifier // 当前所有的验证者
	VerifierNo int              // 验证者所处编号 如果为-1  表示不是验证者
	CurVerfier model.Verifier
	View       uint64 // 当前视图
}

func New() *WroldState {
	//todo:: 加载数据库 加载到最新的ws

	return &WroldState{}
}
