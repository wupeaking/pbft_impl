package world_state

import (
	"fmt"
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
	meta, err := ws.db.GetBlockMeta()
	if err != nil {
		return meta, err
	}
	if meta == nil {
		return nil, fmt.Errorf("block_meta不存在")
	}
	ws.BlockNum = meta.BlockHeight
	ws.View = meta.LastView
	ws.CurVerfier = meta.CurVerfier
	ws.VerifierNo = int(meta.VerifierNo)
	ws.Verifiers = meta.Verifiers
	ws.updateVerifierMap()
	if ws.BlockNum == 0 {
		ws.BlockID = model.GenesisBlockId
		return meta, nil
	}
	blk, err := ws.GetBlock(ws.BlockNum)
	ws.BlockID = blk.BlockId
	ws.PrevBlock = blk.PrevBlock
	return meta, err
}

func (ws *WroldState) GetBlock(key interface{}) (*model.PbftBlock, error) {
	switch x := key.(type) {
	case int, uint, int64, uint64:
		switch reflect.ValueOf(key).Kind() {
		case reflect.Int:
			return ws.db.GetBlockByNum(uint64(key.(int)))
		case reflect.Uint:
			return ws.db.GetBlockByNum(uint64(key.(uint)))
		case reflect.Int64:
			return ws.db.GetBlockByNum(uint64(key.(int64)))
		case reflect.Uint64:
			return ws.db.GetBlockByNum(uint64(key.(uint64)))
		}

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
