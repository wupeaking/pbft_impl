package consensus

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/wupeaking/pbft_impl/model"
)

func (pbft *PBFT) packageBlock() (*model.PbftBlock, error) {
	// 尝试打包一个新区块
	// 当前
	blk := &model.PbftBlock{
		PrevBlock: pbft.ws.BlockID,
		SignerId:  pbft.ws.CurVerfier.PublickKey,
		BlockNum:  pbft.ws.BlockNum + 1,
		TimeStamp: uint64(time.Now().Unix()),
	}
	txs := make([]*model.Tx, 0)
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
	body, err := json.Marshal(txs)
	if err != nil {
		return nil, err
	}
	blk.Content = body
	return pbft.signBlock(blk)
}

// CommitBlock 提交区块
func (pbft *PBFT) CommitBlock(block *model.PbftBlock) error {
	if pbft.ws.BlockNum+1 != block.BlockNum {
		return fmt.Errorf("apply block 失败 区块编号不连续 当前最高区块为: %d", pbft.ws.BlockNum)
	}
	pbft.ws.IncreaseBlockNum()
	pbft.ws.SetValue(block.BlockNum, pbft.ws.BlockID, string(block.BlockId), nil)
	pbft.ws.InsertBlock(block)
	pbft.ws.UpdateLastWorldState()
	pbft.logger.Infof("提交一个新区块")
	pbft.sm.receivedBlock = nil
	return nil
}

// ApplyBlock 执行区块变更
func (pbft *PBFT) ApplyBlock(block *model.PbftBlock) error {
	// todo::
	// 执行区块交易 更新状态信息
	pbft.ws.SetBlockNum(block.BlockNum)
	return nil
}

// TryApplyBlock 执行区块变更 生成新的状态变更
func (pbft *PBFT) TryApplyBlock(block *model.PbftBlock) error {
	// todo::
	// 执行区块交易 更新状态信息
	return nil
}
