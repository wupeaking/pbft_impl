package transaction

import "github.com/wupeaking/pbft_impl/model"

type TxPool struct {
}

func NewTxPool() *TxPool {
	return &TxPool{}
}

func (txpool *TxPool) GetTx() *model.Tx {
	return nil
}

func (txpool *TxPool) RemoveTx(*model.Tx) {

}
