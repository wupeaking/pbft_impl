package world_state

import "github.com/wupeaking/pbft_impl/model"

func (ws *WroldState) InsertBlock(block *model.PbftBlock) error {
	return ws.db.Insert(block)
}

func (ws *WroldState) UpdateLastWorldState() error {
	return ws.db.Insert(&model.BlockMeta{
		BlockHeight: ws.BlockNum,
		CurVerfier:  ws.CurVerfier,
		VerifierNo:  uint32(ws.VerifierNo),
		Verifiers:   ws.Verifiers,
		LastView:    ws.View,
	})
}
