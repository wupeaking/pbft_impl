package consensus

import (
	"bytes"
	"fmt"
	"time"

	cryptogo "github.com/wupeaking/pbft_impl/crypto"
	"github.com/wupeaking/pbft_impl/model"
)

func (pbft *PBFT) packageBlock() (*model.PbftBlock, error) {
	privKey, err := cryptogo.LoadPrivateKey(fmt.Sprintf("0x%x", pbft.ws.CurVerfier.PrivateKey))
	if err != nil {
		return nil, err
	}

	// 尝试打包一个新区块
	blk := &model.PbftBlock{
		PrevBlock:      pbft.ws.BlockID,
		SignerId:       pbft.ws.CurVerfier.PublickKey,
		BlockNum:       pbft.ws.BlockNum + 1,
		TimeStamp:      uint64(time.Now().Unix()),
		View:           pbft.ws.View,
		TxRoot:         nil,
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
	if len(txs) != 0 {
		pbft.logger.Debugf("本次打包的交易数量为: %d", len(txs))
	}
	blk.Tansactions = &model.Txs{Tansactions: txs}
	// todo:: 需要调用执行txs模块 生成blk.TransactionReceipts
	blk.TransactionReceipts = &model.TxReceipts{TansactionReceipts: make([]*model.TxReceipt, 0)}
	for i := range blk.Tansactions.Tansactions {
		txr := pbft.vm.Eval(blk.Tansactions.Tansactions[i])
		err := txr.SignedTxReceipt(privKey)
		if err != nil {
			return nil, err
		}
		blk.TransactionReceipts.TansactionReceipts = append(blk.TransactionReceipts.TansactionReceipts, txr)
	}
	blk.TxRoot = blk.Tansactions.MerkleRoot()
	blk.TxReceiptsRoot = blk.TransactionReceipts.MerkleRoot()

	return pbft.signBlock(blk)
}

// CommitBlock 提交区块
func (pbft *PBFT) CommitBlock(block *model.PbftBlock) error {
	if pbft.ws.BlockNum+1 != block.BlockNum {
		return fmt.Errorf("commit block 失败 区块编号不连续 当前最高区块为: %d", pbft.ws.BlockNum)
	}
	if pbft.ws.BlockID != block.PrevBlock {
		return fmt.Errorf("commit block 失败 提交的新区块指向的前区块不一致 新区块指向的id为: %s 本地的区块id为: %s",
			block.PrevBlock, pbft.ws.BlockID)
	}
	if err := pbft.ApplyBlock(block); err != nil {
		return err
	}

	pbft.ws.IncreaseBlockNum()
	pbft.ws.SetValue(block.BlockNum, pbft.ws.BlockID, block.BlockId, nil)
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
	// todo:: 还是需要涉及到交易回退问题 可能需要有交易快照功能
	//1. 执行区块交易 更新状态信息
	//2. 删除交易池的交易
	for _, tx := range block.Tansactions.Tansactions {
		_, err := pbft.vm.Exec(tx)
		if err != nil {
			return err
		}
		pbft.txPool.RemoveTx(tx)
	}
	//pbft.ws.SetBlockNum(block.BlockNum)
	// pbft.ws.SetValue(block.BlockNum, pbft.ws.BlockID, string(block.BlockId), nil)
	return nil
}

// TryApplyBlock  尝试执行区块交易 校验接收到区块交易是否正确
func (pbft *PBFT) TryApplyBlock(block *model.PbftBlock) error {
	// 1. txs数量应该和txreceipt数量一致
	// 2. txs应该是合法交易
	// 3. txreciept 应该是合法的
	// 4. merker树应该是正确的
	if len(block.Tansactions.Tansactions) != len(block.TransactionReceipts.TansactionReceipts) {
		return fmt.Errorf("区块交易数量和收据数量不一致")
	}
	txs := map[string]struct{}{}
	for i := range block.Tansactions.Tansactions {
		if !block.Tansactions.Tansactions[i].IsVaildTx() {
			return fmt.Errorf("交易信息不合法")
		}
		if _, ok := txs[string(block.Tansactions.Tansactions[i].Sign)]; ok {
			return fmt.Errorf("包含重复交易")
		}
		txs[string(block.Tansactions.Tansactions[i].Sign)] = struct{}{}
	}
	for i, txr := range block.TransactionReceipts.TansactionReceipts {
		if bytes.Compare(block.Tansactions.Tansactions[i].Sign, txr.TxId) != 0 {
			return fmt.Errorf("交易收据与交易不能对应")
		}
		if !txr.IsVaildTxR(block.SignerId) {
			return fmt.Errorf("交易收据信息不合法")
		}

		// todo:: 模拟执行 有些问题点未解决
		// 模拟执行 需要涉及到整个区块交易的状态变更 但是又不能更新状态
		// txrRet := pbft.vm.Eval(block.Tansactions.Tansactions[i])
		// if txrRet.Status != txr.Status {
		// 	return fmt.Errorf("预执行交易不一致")
		// }
	}

	if bytes.Compare(block.Tansactions.MerkleRoot(), block.TxRoot) != 0 {
		return fmt.Errorf("交易的默克尔树校验不一致")
	}

	if bytes.Compare(block.TransactionReceipts.MerkleRoot(), block.TxReceiptsRoot) != 0 {
		return fmt.Errorf("交易收据的默克尔树校验不一致")
	}
	return nil
}
