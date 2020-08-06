package consensus

import (
	"encoding/json"
	"fmt"

	"github.com/wupeaking/pbft_impl/model"
	"github.com/wupeaking/pbft_impl/transaction"
)

func (pbft *PBFT) packageBlock() (*model.PbftBlock, error) {
	// 尝试打包一个新区块
	// 当前
	blk := &model.PbftBlock{
		BlockId:  []byte(pbft.ws.BlockID),
		SignerId: pbft.ws.CurVerfier.PublickKey,
	}
	txs := make([]*transaction.Tx, 0)
	max := 3000
	i := 0
	for {
		t := pbft.txPool.GetTx()
		if t == nil {
			break
		}
		if i > max {
			break
		}
		txs = append(txs, t)
	}
	body, _ := json.Marshal(txs)
	blk.Content = body
	return pbft.signBlock(blk)
}

func (pbft *PBFT) ApplyBlock(block *model.PbftBlock) error {
	if pbft.ws.BlockNum+1 != block.BlockNum {
		return fmt.Errorf("apply block 失败 区块编号不连续 当前最高区块为: %d", pbft.ws.BlockNum)
	}
	pbft.ws.IncreaseBlockNum()
	pbft.ws.SetValue(0, pbft.ws.BlockID, string(block.BlockId), nil)
	pbft.ws.InsertBlock(block)
	return nil
}
