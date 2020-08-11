package world_state

import (
	"reflect"

	"github.com/wupeaking/pbft_impl/model"
)

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

func (ws *WroldState) GetBlockMeta() (*model.BlockMeta, error) {
	return ws.db.GetBlockMeta()
}

func (ws *WroldState) GetBlock(key interface{}) (*model.PbftBlock, error) {
	switch x := key.(type) {
	case int, uint, int64, uint64:
		v := reflect.ValueOf(key).Uint()
		return ws.db.GetBlockByNum(v)
	case string:
		return ws.db.GetBlockByID(x)
	case []byte:
		return ws.db.GetBlockByID(string(x))
	}
	return nil, nil
}

func (ws *WroldState) GetGenesis() (*model.Genesis, error) {
	return ws.db.GetGenesisBlock()
}

func (ws *WroldState) SetGenesis(g *model.Genesis) error {
	return ws.db.SetGenesisBlock(g)
}
