package cvm

import (
	"fmt"

	"github.com/wupeaking/pbft_impl/model"
)

type Snapshot struct {
	txs      map[string]*model.Tx
	accounts map[string]*model.Account
}

// 临时缓存区 在模拟执行时 需要将账户和交易状态放入此区域 防止错误模拟
func NewSnapshot() *Snapshot {
	return &Snapshot{
		txs:      map[string]*model.Tx{},
		accounts: map[string]*model.Account{},
	}
}

func (ss *Snapshot) GetTxByID(id string) *model.Tx {
	return ss.txs[id]
}

func (ss *Snapshot) GetAccountByID(id string) *model.Account {
	return ss.accounts[id]
}

func (ss *Snapshot) UpdateAccountByID(acc *model.Account) {
	ss.accounts[acc.Id.Address] = acc
}

func (ss *Snapshot) UpdateTxByID(tx *model.Tx) {
	ss.txs[fmt.Sprintf("%0x", tx.Sign)] = tx
}
