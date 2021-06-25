package transaction

import (
	"bytes"
	"testing"

	"github.com/wupeaking/pbft_impl/model"
)

func TestPools(t *testing.T) {
	pool := NewPool(10)
	pool.addValue(&model.Tx{Sign: []byte{1}})
	pool.addValue(&model.Tx{Sign: []byte{2}})
	pool.addValue(&model.Tx{Sign: []byte{3}})
	pool.addValue(&model.Tx{Sign: []byte{4}})
	pool.addValue(&model.Tx{Sign: []byte{5}})
	pool.addValue(&model.Tx{Sign: []byte{6}})

	txs := pool.scanValue(3)
	if len(txs) != 3 {
		t.Fatalf("scan 错误 len(txs) = %d", len(txs))
	}
	for i := range txs {
		if bytes.Compare([]byte{byte(i + 1)}, txs[i].Sign) != 0 {
			t.Fatalf("scan value错误 value: %v", txs[i].Sign)
		}
	}
	pool.delValue(&model.Tx{Sign: []byte{3}})
	pool.delValue(&model.Tx{Sign: []byte{1}})
	pool.delValue(&model.Tx{Sign: []byte{2}})
	pool.delValue(&model.Tx{Sign: []byte{6}})
	txs = pool.scanValue(10)
	if len(txs) != 2 {
		t.Fatalf("scan 错误 len(txs) = %d", len(txs))
	}
	for i := range txs {
		if bytes.Compare([]byte{byte(i + 4)}, txs[i].Sign) != 0 {
			t.Fatalf("scan value错误 value: %v", txs[i].Sign)
		}
	}
	// 删除所有
	pool.delValue(&model.Tx{Sign: []byte{5}})
	pool.delValue(&model.Tx{Sign: []byte{4}})

	pool.addValue(&model.Tx{Sign: []byte{4}})
	pool.addValue(&model.Tx{Sign: []byte{5}})
	pool.addValue(&model.Tx{Sign: []byte{6}})
	pool.addValue(&model.Tx{Sign: []byte{6}})
	txs = pool.scanValue(10)
	if len(txs) != 3 {
		t.Fatalf("scan 错误 len(txs) = %d", len(txs))
	}
	for i := range txs {
		if bytes.Compare([]byte{byte(i + 4)}, txs[i].Sign) != 0 {
			t.Fatalf("scan value错误 value: %v", txs[i].Sign)
		}
	}
}
