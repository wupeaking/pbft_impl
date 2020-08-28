package node

import (
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/wupeaking/pbft_impl/blockchain"
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
	chain           *blockchain.BlockChain
	tx              *transaction.TxPool
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

	switcher := http_network.New(cfg.NodeAddrs, cfg.LocalAddr, cfg.NetworkCfg.Publickey)
	txPool := transaction.NewTxPool(switcher)

	var consen *consensus.PBFT

	if genesis == nil {
		// 生成创世区块
		var pub, pri []byte
		if strings.HasPrefix(cfg.ConsensusCfg.Publickey, "0x") || strings.HasPrefix(cfg.ConsensusCfg.Publickey, "0x") {
			pub, _ = hex.DecodeString(cfg.ConsensusCfg.Publickey[2:])
		}
		if strings.HasPrefix(cfg.ConsensusCfg.PriVateKey, "0x") || strings.HasPrefix(cfg.ConsensusCfg.PriVateKey, "0x") {
			pri, _ = hex.DecodeString(cfg.ConsensusCfg.PriVateKey[2:])
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
			if fmt.Sprintf("0x%x", ws.Verifiers[i].PublickKey) == strings.ToLower(cfg.ConsensusCfg.Publickey) {
				pub, _ := cryptogo.Hex2Bytes(cfg.ConsensusCfg.Publickey)
				pri, _ := cryptogo.Hex2Bytes(cfg.ConsensusCfg.PriVateKey)
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

	chain := blockchain.New(consen, ws, switcher)

	return &PBFTNode{
		consensusEngine: consen,
		switcher:        switcher,
		ws:              ws,
		chain:           chain,
		tx:              txPool,
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
	//// todo:: 如果不是验证者 暂时还不能运行
	if node.ws.CurVerfier == nil {
		logger.Fatalf("当前节点不是验证者, 暂时不能启动")
	}

	// 启动P2P
	go node.switcher.Start()
	// 启动共识
	go node.consensusEngine.Daemon()
	// 启动Blockchain
	go node.chain.Start()
	// 启动交易池
	go node.tx.Start()

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

	select {}
}
