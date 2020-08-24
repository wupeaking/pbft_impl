package node

import (
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/wupeaking/pbft_impl/consensus"
	cryptogo "github.com/wupeaking/pbft_impl/crypto"
	"github.com/wupeaking/pbft_impl/model"
	"github.com/wupeaking/pbft_impl/network"
	"github.com/wupeaking/pbft_impl/network/http_network"
	"github.com/wupeaking/pbft_impl/storage/world_state"
	"github.com/wupeaking/pbft_impl/transaction"
)

// Node
type PBFTNode struct {
	consensusEngine *consensus.PBFT
	switcher        network.SwitcherI
	ws              *world_state.WroldState
}

func New() *PBFTNode {
	// 检查本地是否已经保存数据
	ws := world_state.New("./.counch")
	// 读取创世区块
	genesis, err := ws.GetGenesis()
	if err != nil {
		logger.Fatalf("读取创世区块发生错误 err: %v", err)
	}

	// 尝试读取配置文件
	cfg, err := LoadConfig("./.counch/config.json")
	if err != nil {
		logger.Fatalf("读取配置文件发生错误 err: %v", err)
	}
	if cfg == nil {
		logger.Warnf("未读取到配置文件  尝试使用默认配置")
		cfg, _ = DefaultConfig()
	}

	switcher := http_network.New(cfg.NodeAddrs, cfg.LocalAddr)
	txPool := transaction.NewTxPool()

	var consen *consensus.PBFT

	if genesis == nil {
		// 生成创世区块
		var pub, pri []byte
		if strings.HasPrefix(cfg.Publickey, "0x") || strings.HasPrefix(cfg.Publickey, "0x") {
			pub, _ = hex.DecodeString(cfg.Publickey[2:])
		}
		if strings.HasPrefix(cfg.PriVateKey, "0x") || strings.HasPrefix(cfg.PriVateKey, "0x") {
			pri, _ = hex.DecodeString(cfg.PriVateKey[2:])
		}
		zeroBlock := model.Genesis{
			Verifiers: []*model.Verifier{
				{
					PublickKey: pub,
					PrivateKey: nil,
					SeqNum:     0,
				},
			},
		}
		ws.CurVerfier = &model.Verifier{PublickKey: pub, PrivateKey: pri, SeqNum: 0}
		ws.VerifierNo = 0
		ws.SetGenesis(&zeroBlock)
		ws.SetValue(0, "", "genesis", zeroBlock.Verifiers)
		ws.UpdateLastWorldState()

		pbft, err := consensus.New(ws, txPool, switcher)
		if err != nil {
			logger.Fatalf("读取配置文件发生错误 err: %v", err)
		}
		consen = pbft
		// cfg.ConsensusCfg.Publickey
		// cfg.ConsensusCfg.PriVateKey
	} else {
		ws.Verifiers = genesis.Verifiers
		isVerfier := false
		for i := range ws.Verifiers {
			if fmt.Sprintf("0x%x", ws.Verifiers[i].PublickKey) == strings.ToLower(cfg.Publickey) {
				pub, _ := cryptogo.Hex2Bytes(cfg.Publickey)
				pri, _ := cryptogo.Hex2Bytes(cfg.PriVateKey)
				ws.CurVerfier = &model.Verifier{PublickKey: pub, PrivateKey: pri, SeqNum: int32(i)}
				ws.VerifierNo = i
				isVerfier = true
				break
			}
		}
		if isVerfier {
			logger.Infof("当前节点是验证者, 编号为: %d", ws.VerifierNo)
		} else {
			ws.VerifierNo = -1
			logger.Infof("当前节点不是验证者, 作为普通节点启动")
		}

		pbft, err := consensus.New(ws, txPool, switcher)
		if err != nil {
			logger.Fatalf("读取配置文件发生错误 err: %v", err)
		}
		consen = pbft
	}

	return &PBFTNode{
		consensusEngine: consen,
		switcher:        switcher,
		ws:              ws,
	}
}

func (node *PBFTNode) Run() {
	// 获取blockmeta
	meta, err := node.ws.GetBlockMeta()
	if err != nil {
		logger.Fatalf("读取区块元数据错误 err: %v", err)
	}
	if meta.BlockHeight > node.ws.BlockNum {
		// 如果当前状态还未达到最高 需要apply
		for i := uint64(1); i < meta.BlockHeight; i++ {
			blk, err := node.ws.GetBlock(i)
			if err != nil {
				logger.Fatalf("读取%d 区块出错 err: %v", err)
			}
			if blk == nil {
				logger.Fatalf("读取%d 区块为空 但是block Meta存在")
			}
			node.consensusEngine.ApplyBlock(blk)
		}
	}
	// 如果不是验证者
	{
		// todo::
	}

	//1. 如何知道自己处于最高区块高度
	// 广播获取最高区块高度
	// 要求在规定时间内 收到指定的区块高度
	//	判断是否已经大于最高区块高度  如果没有则 停止共识 接收下载区块高度
	// 循环知道达到最高区块高度

	// 启动共识
	go node.consensusEngine.Daemon()

	curBlock := 0
	go func() {
		for {
			time.Sleep(1000 * time.Millisecond)
			if node.consensusEngine.StopFlag == false && node.consensusEngine.CurrentState() == model.States_NotStartd && node.ws.BlockNum >= uint64(curBlock) {
				newMsg := model.PbftGenericMessage{
					Info: &model.PbftMessageInfo{MsgType: model.MessageType_NewBlockProposal,
						View: node.ws.View, SeqNum: node.ws.BlockNum + 1,
						SignerId: node.ws.CurVerfier.PublickKey,
						Sign:     nil,
					},
				}
				// 签名
				signedMsg, err := node.consensusEngine.SignMsg(model.NewPbftMessage(&newMsg))
				if err != nil {
					logger.Errorf("新区块提议消息签名失败, err: %v", err)
					continue
				}
				node.consensusEngine.Msgs.InsertMsg(signedMsg)
			}
		}
	}()

	for msg := range node.switcher.Recv() {
		switch x := msg.(type) {
		case *model.BlockMeta:
			// 校验BlockMeta
			// 判断当前节点是否是处于最高区块
			if x.BlockHeight > node.ws.BlockNum {
				// 停止共识
				// 再次走到判断是否是最高区块高度流程
				break
			} else {

			}
		case *model.PbftMessage:
			node.consensusEngine.Msgs.InsertMsg(x)
		}
	}

}
