package transaction

import (
	"fmt"
	"sync"

	"github.com/wupeaking/pbft_impl/model"
)

// 实现一个交易池的功能
// 1. 能够进行追加 删除 查找 复杂度要在O(1)
// 2. 控制容量大小
// 3. 需要能遍历 并且记录上次遍历的位置
// 暂定使用双向链表实现

type node struct {
	prev  *node
	next  *node
	value *model.Tx
}

type Pool struct {
	cursor *node
	tail   *node
	cap    uint64
	length uint64
	sync.RWMutex
	txids map[string]*node
}

func NewPool(size uint64) *Pool {
	return &Pool{
		cap:   size,
		txids: make(map[string]*node),
	}
}

func (p *Pool) len() uint64 {
	return p.length
}

func (p *Pool) addValue(tx *model.Tx) bool {
	p.Lock()
	defer p.Unlock()
	txid := fmt.Sprintf("%0x", tx.Sign)
	_, ok := p.txids[txid]
	if ok {
		// 如果已经存在 则不添加
		return false
	}

	if p.cap <= p.length {
		return false
	}

	n := &node{
		value: tx,
	}
	p.txids[txid] = n
	if p.tail == nil {
		p.tail = n
	} else {
		cur := p.tail
		cur.next = n
		n.prev = cur
		p.tail = n
	}
	if p.cursor == nil {
		p.cursor = p.tail
	}
	p.length++
	return true
}

// scanValue  读取num个值 读取会移动游标 不能读取到重复的值
func (p *Pool) scanValue(num int) []*model.Tx {
	p.RLock()
	defer p.RUnlock()
	txs := make([]*model.Tx, 0)
	i := 0
	for p.cursor != nil && i < num {
		n := p.cursor
		p.cursor = p.cursor.next
		txs = append(txs, n.value)
		i++
	}
	return txs
}

func (p *Pool) delValue(tx *model.Tx) {
	p.Lock()
	defer p.Unlock()
	txid := fmt.Sprintf("%0x", tx.Sign)
	n, ok := p.txids[txid]
	if !ok {
		return
	}
	delete(p.txids, txid)

	pre := n.prev
	next := n.next
	if pre != nil {
		pre.next = next
	}
	if next != nil {
		next.prev = pre
	}
	if p.tail == n {
		p.tail = n.next
	}
	if p.cursor == n {
		p.cursor = n.next
	}
	p.length--
	return
}
