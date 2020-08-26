package blockchain

import (
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/wupeaking/pbft_impl/model"
	"github.com/wupeaking/pbft_impl/network"
	"github.com/wupeaking/pbft_impl/storage/world_state"
)

type BlockPool struct {
	switcher    network.SwitcherI
	ws          *world_state.WroldState
	heightPeers map[uint64][]*network.Peer
	numBlock    map[uint64]*model.PbftBlock
	newBlock    chan *model.PbftBlock
	addBlock    chan *model.PbftBlock
	stopEngine  chan struct{}
	startEngine chan struct{}
	sync.RWMutex
	maxHeight       uint64
	requestComplate map[uint64]chan struct{}
}

func (bp *BlockPool) downRoutine(num uint64) {
	request := model.BlockRequest{
		RequestType: model.BlockRequestType_whole_content,
		BlockNum:    int64(num),
	}
	body, _ := proto.Marshal(&request)
	msg := network.BroadcastMsg{
		ModelID: "blockchain",
		MsgType: model.BroadcastMsgType_request_load_block,
		Msg:     body,
	}
	complatedSig := make(chan struct{}, 1)
	bp.requestComplate[num] = complatedSig

	peer := bp.pickPeer(num, nil)
	bp.switcher.BroadcastToPeer("blockchain", &msg, peer)

	timeout := time.NewTimer(5 * time.Second)

	for {
		select {
		case <-complatedSig:
			delete(bp.requestComplate, num)
			return
		case <-timeout.C:
			// 重新挑选一个peer  再次广播
			peer = bp.pickPeer(num, peer)
			bp.switcher.BroadcastToPeer("blockchain", &msg, peer)
			timeout.Reset(5 * time.Second)
		}
	}
}

func (bp *BlockPool) pickPeer(blockNum uint64, oldPeer *network.Peer) *network.Peer {
	/// 从 heightPeers中找出比blockNum高的 然后和oldpeer不重复的
	for num, peers := range bp.heightPeers {
		if num < blockNum {
			continue
		}
		for _, p := range peers {
			if oldPeer == nil {
				return p
			}
			if oldPeer.ID != p.ID {
				return p
			}
		}
	}
	return nil
}

func NewBlockPool(ws *world_state.WroldState, switcher network.SwitcherI) *BlockPool {
	return &BlockPool{
		switcher:        switcher,
		ws:              ws,
		heightPeers:     make(map[uint64][]*network.Peer),
		newBlock:        make(chan *model.PbftBlock),
		addBlock:        make(chan *model.PbftBlock),
		startEngine:     make(chan struct{}, 1),
		stopEngine:      make(chan struct{}, 1),
		requestComplate: make(map[uint64]chan struct{}),
	}
}

func (bp *BlockPool) SetPeerHight(peer *network.Peer, height uint64) {
	if bp.ws.BlockNum >= height {
		return
	}
	// 说明本节点已经落后 停止共识 追上最高节点
	if bp.maxHeight < height {
		bp.maxHeight = height
	}
}

func (bp *BlockPool) AddBlock(peer *network.Peer, block *model.PbftBlock) {
	if bp.ws.BlockNum >= block.BlockNum {
		return
	}
	bp.addBlock <- block

	complate := bp.requestComplate[block.BlockNum]
	if complate != nil {
		select {
		case complate <- struct{}{}:
		default:
		}
	}
}

func (bp *BlockPool) RemoveBlock(block *model.PbftBlock) {
	bp.Lock()
	delete(bp.numBlock, block.BlockNum)
	bp.Unlock()
}

func (bp *BlockPool) Routine() {
	trySyncTicker := time.NewTicker(1 * time.Second)
	stateTicker := time.NewTicker(500 * time.Millisecond)
	for {
		select {
		case <-trySyncTicker.C:
			// 尝试请求最高区块
			bp.requestBlockHeight()
		case <-stateTicker.C:
			if bp.ws.BlockNum >= bp.maxHeight {
				select {
				case bp.startEngine <- struct{}{}:
				default:
				}

			} else {
				select {
				case bp.stopEngine <- struct{}{}:
				default:
				}
				// todo:: 下载区块
			}
		case block := <-bp.addBlock:
			if bp.ws.BlockNum+1 == block.BlockNum {
				bp.newBlock <- block
				nextnum := bp.ws.BlockNum + 2
				for {
					// 尝试查看下一个区块是否已经存在
					b, ok := bp.numBlock[nextnum]
					if ok {
						bp.newBlock <- b
						nextnum++
					} else {
						break
					}
				}
			} else {
				bp.Lock()
				bp.numBlock[block.BlockNum] = block
				bp.Unlock()
			}
		}
	}
}

func (bp *BlockPool) requestBlockHeight() {
	request := model.BlockRequest{
		RequestType: model.BlockRequestType_only_header,
		BlockNum:    -1,
	}
	body, _ := proto.Marshal(&request)
	msg := network.BroadcastMsg{
		ModelID: "blockchain",
		MsgType: model.BroadcastMsgType_request_load_block,
		Msg:     body,
	}
	bp.switcher.Broadcast("blockchain", &msg)
}
