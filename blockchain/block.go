package blockchain

import (
	"encoding/json"

	"github.com/golang/protobuf/proto"
	log "github.com/sirupsen/logrus"
	"github.com/wupeaking/pbft_impl/consensus"
	"github.com/wupeaking/pbft_impl/model"
	"github.com/wupeaking/pbft_impl/network"
	"github.com/wupeaking/pbft_impl/storage/world_state"
)

var logger *log.Entry

func init() {
	logg := log.New()
	logg.SetLevel(log.DebugLevel)
	logg.SetReportCaller(true)
	logg.SetFormatter(&log.TextFormatter{
		DisableColors: true,
		FullTimestamp: true,
	})
	logger = logg.WithField("module", "blockchain")
}

//BlockChain 的作用 定时更新区块高度 随时启动和暂停共识引擎

type BlockChain struct {
	consensusEngine *consensus.PBFT
	ws              *world_state.WroldState
	switcher        network.SwitcherI
	pool            *BlockPool
}

func New(c *consensus.PBFT, ws *world_state.WroldState, switcher network.SwitcherI) *BlockChain {
	return &BlockChain{
		consensusEngine: c,
		ws:              ws,
		switcher:        switcher,
		pool:            NewBlockPool(ws, switcher),
	}
}

func (bc *BlockChain) Start() error {
	if err := bc.switcher.RegisterOnReceive("blockchain", bc.msgOnRecv); err != nil {
		return err
	}

	go bc.pool.Routine()

	for {
		select {
		case block := <-bc.pool.newBlock:
			// 新的区块来到了
			if bc.ws.BlockNum+1 > block.BlockNum {
				bc.pool.RemoveBlock(block)
			}
			if bc.ws.BlockNum+1 < block.BlockNum {
				continue
			}
			if bc.consensusEngine.ApplyBlock(block) != nil {
				continue
			}
			bc.consensusEngine.CommitBlock(block)
			bc.pool.RemoveBlock(block)
		case <-bc.pool.startEngine:
			bc.consensusEngine.Start()
		case <-bc.pool.stopEngine:
			bc.consensusEngine.Stop()
		}

	}

}

func (bc *BlockChain) msgOnRecv(modelID string, msgBytes []byte, p *network.Peer) {
	if modelID != "blockchain" {
		return
	}
	var msgPkg network.BroadcastMsg
	if json.Unmarshal(msgBytes, &msgPkg) != nil {
		return
	}

	switch msgPkg.MsgType {
	case model.BroadcastMsgType_request_load_block:
		// 表示对方请求本节点的区块信息
		var blockReq *model.BlockRequest
		if proto.Unmarshal(msgPkg.Msg, blockReq) != nil {
			return
		}
		blockNum := blockReq.BlockNum
		if blockNum == -1 {
			// 则认为是想获取最高区块高度
			blockNum = int64(bc.ws.BlockNum)
		}
		blk, err := bc.ws.GetBlock(blockNum)
		if err != nil {
			logger.Warnf("依靠区块标号查询区块出错 err: %v", err)
			return
		}
		if blk == nil {
			return
		}

		if blockReq.RequestType == model.BlockRequestType_only_header {
			// 只发送区块头
			blk.Content = nil
		}
		resp := model.BlockResponse{RequestType: blockReq.RequestType, Block: blk}
		body, _ := proto.Marshal(&resp)
		msg := network.BroadcastMsg{
			ModelID: "blockchain",
			MsgType: model.BroadcastMsgType_send_specific_block,
			Msg:     body,
		}
		err = bc.switcher.BroadcastToPeer("blockchain", &msg, p)
		if err != nil {
			//todo:: 可能需要移除这个peer
			bc.switcher.RemovePeer(p)
		}
		return

	case model.BroadcastMsgType_send_specific_block:
		// 表示对方向本节点发送区块信息
		var blockResp *model.BlockResponse
		if proto.Unmarshal(msgPkg.Msg, blockResp) != nil {
			return
		}
		if blockResp.RequestType == model.BlockRequestType_only_header {
			// 校验区块头
			if !bc.consensusEngine.VerfifyBlockHeader(blockResp.Block) {
				return
			}
			// 判断高度
			if bc.ws.BlockNum >= blockResp.Block.BlockNum {
				return
			}
			bc.pool.SetPeerHight(p, blockResp.Block.BlockNum)
		} else {
			if !bc.consensusEngine.VerfifyMostBlock(blockResp.Block) {
				return
			}
			bc.pool.AddBlock(p, blockResp.Block)
		}

	}

}
