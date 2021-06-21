package consensus

import (
	"fmt"
	"time"

	"github.com/wupeaking/pbft_impl/model"
)

func (pbft *PBFT) packageBlock() (*model.PbftBlock, error) {
	// todo:: 后期 打包区块 需要做更多的功能 他需要执行区块的交易 生成tx_root tx_recpt_root
	// 尝试打包一个新区块
	// 当前
	blk := &model.PbftBlock{
		PrevBlock: pbft.ws.BlockID,
		SignerId:  pbft.ws.CurVerfier.PublickKey,
		BlockNum:  pbft.ws.BlockNum + 1,
		TimeStamp: uint64(time.Now().Unix()),
		View:      pbft.ws.View,
		// todo:: 需要根据txs生成tx_root
		TxRoot: nil,
		// todo:: 需要根据tx_recpts生成
		TxReceiptsRoot: nil,
	}
	txs := make([]*model.Tx, 0)
	max := 3000
	for {
		ts := pbft.txPool.GetTx(max)
		if len(ts) == 0 {
			break
		}
		if len(txs)+len(ts) > max {
			break
		}
		txs = append(txs, ts...)
	}
	blk.Tansactions = &model.Txs{Tansactions: txs}
	// todo:: 需要调用执行txs模块 生成blk.TransactionReceipts
	blk.TransactionReceipts = nil
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
	// 更新视图
	pbft.ws.View = block.View
	pbft.ws.UpdateLastWorldState()
	pbft.logger.Infof("提交一个新区块, 区块高度为: %d", block.GetBlockNum())
	pbft.sm.receivedBlock = nil
	return nil
}

// ApplyBlock 执行区块变更
func (pbft *PBFT) ApplyBlock(block *model.PbftBlock) error {
	// todo::
	// 执行区块交易 更新状态信息
	//pbft.ws.SetBlockNum(block.BlockNum)
	pbft.ws.SetValue(block.BlockNum, pbft.ws.BlockID, string(block.BlockId), nil)
	return nil
}

// TryApplyBlock 执行区块变更 生成新的状态变更
func (pbft *PBFT) TryApplyBlock(block *model.PbftBlock) error {
	// todo::
	// 执行区块交易 更新状态信息
	return nil
}
