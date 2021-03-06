package node

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/wupeaking/pbft_impl/account"
	"github.com/wupeaking/pbft_impl/api"
	"github.com/wupeaking/pbft_impl/blockchain"
	"github.com/wupeaking/pbft_impl/common/config"
	"github.com/wupeaking/pbft_impl/consensus"
	cryptogo "github.com/wupeaking/pbft_impl/crypto"
	"github.com/wupeaking/pbft_impl/cvm"
	"github.com/wupeaking/pbft_impl/model"
	"github.com/wupeaking/pbft_impl/network"
	"github.com/wupeaking/pbft_impl/network/http_network"
	"github.com/wupeaking/pbft_impl/network/libp2p"
	"github.com/wupeaking/pbft_impl/storage/cache"
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
	vm              *cvm.VirtualMachine
	apiServer       *api.API
	db              *cache.DBCache
}

func New() *PBFTNode {
	// 创建缓存数据库
	db := cache.New("./.counch")
	// 检查本地是否已经保存数据
	ws := world_state.New(db, ".counch")
	// 读取创世区块
	genesis, err := ws.GetGenesis()
	if err != nil {
		logger.Fatalf("读取创世区块发生错误 err: %v", err)
	}

	// 尝试读取配置文件
	cfg, err := config.LoadConfig("./.counch/config.json")
	if err != nil {
		logger.Fatalf("读取配置文件发生错误 err: %v", err)
	}
	var switcher network.SwitcherI
	if cfg.NetMode == "http" {
		switcher = http_network.New(cfg.NodeAddrs, cfg.LocalAddr, cfg.NetworkCfg.Publickey, cfg)
	} else {
		switcher, err = libp2p.New(cfg)
		if err != nil {
			panic(err)
		}
	}

	vm := cvm.New(db, cfg)
	txPool := transaction.NewTxPool(switcher, cfg, db)

	var consen *consensus.PBFT

	if genesis == nil {
		logger.Infof("当前未读取到本地创世区块, 使用配置文件创建创建创世区块")
		// 生成创世区块
		if len(cfg.ConsensusCfg.Verfiers) == 0 {
			logger.Fatalf("配置文件内容错误 不能设置空验证者列表")
		}
		zeroBlock := model.Genesis{
			Verifiers: make([]*model.Verifier, 0),
		}

		for i, verfiers := range cfg.Verfiers {
			pub, err := cryptogo.Hex2Bytes(verfiers.Publickey)
			if err != nil {
				logger.Fatalf("验证者公钥格式错误")
			}
			zeroBlock.Verifiers = append(zeroBlock.Verifiers, &model.Verifier{PublickKey: pub, SeqNum: int32(i)})
			if cfg.ConsensusCfg.Publickey == verfiers.Publickey {
				pri, _ := cryptogo.Hex2Bytes(cfg.ConsensusCfg.PriVateKey)
				ws.CurVerfier = &model.Verifier{PublickKey: pub, PrivateKey: pri, SeqNum: 0}
				ws.VerifierNo = i
			}
		}

		if err := ws.SetGenesis(&zeroBlock); err != nil {
			panic(err)
		}
		ws.SetValue(0, "", model.GenesisBlockId, zeroBlock.Verifiers)
		ws.UpdateLastWorldState()

		// 加载预设值的账户
		for i := range cfg.AccountCfg {
			acc := &model.Account{
				Id:          &model.Address{Address: cfg.AccountCfg[i].Address},
				Balance:     &model.Amount{Amount: fmt.Sprintf("%d", cfg.AccountCfg[i].Amount)},
				AccountType: int32(cfg.AccountCfg[i].Type),
			}
			if err := db.Insert(acc); err != nil {
				panic(err)
			}
		}
	}
	// } else {
	// 	logger.Infof("读取到本地创世区块, 本地配置文件某些配置项可能会被覆盖")
	// 	isVerfier := false
	// 	for i := range ws.Verifiers {
	// 		if fmt.Sprintf("0x%x", ws.Verifiers[i].PublickKey) == strings.ToLower(cfg.ConsensusCfg.Publickey) {
	// 			pub, _ := cryptogo.Hex2Bytes(cfg.ConsensusCfg.Publickey)
	// 			pri, _ := cryptogo.Hex2Bytes(cfg.ConsensusCfg.PriVateKey)
	// 			ws.CurVerfier = &model.Verifier{PublickKey: pub, PrivateKey: pri, SeqNum: int32(i)}
	// 			ws.VerifierNo = i
	// 			isVerfier = true
	// 			break
	// 		}
	// 	}
	// 	if isVerfier {
	// 		logger.Infof("当前节点是验证者, 编号为: %d", ws.VerifierNo)
	// 	} else {
	// 		ws.VerifierNo = -1
	// 		logger.Infof("当前节点不是验证者, 作为普通节点启动")
	// 	}

	// 	pbft, err := consensus.New(ws, txPool, switcher, vm, cfg)
	// 	if err != nil {
	// 		logger.Fatalf("读取配置文件发生错误 err: %v", err)
	// 	}
	// 	consen = pbft
	// }
	pbft, err := consensus.New(ws, txPool, switcher, vm, cfg)
	if err != nil {
		logger.Fatalf("读取配置文件发生错误 err: %v", err)
	}
	consen = pbft
	chain := blockchain.New(consen, ws, switcher)
	apiServer := api.New(cfg)

	return &PBFTNode{
		consensusEngine: consen,
		switcher:        switcher,
		ws:              ws,
		chain:           chain,
		tx:              txPool,
		vm:              vm,
		apiServer:       apiServer,
		db:              db,
	}
}

func (node *PBFTNode) Run() {
	// 获取blockmeta 更新ws
	_, err := node.ws.GetBlockMeta()
	if err != nil {
		logger.Fatalf("读取区块元数据错误 err: %v", err)
	}
	// if meta.BlockHeight > node.ws.BlockNum {
	// 	// 如果当前状态还未达到最高 需要apply
	// 	for i := uint64(1); i < meta.BlockHeight; i++ {
	// 		blk, err := node.ws.GetBlock(i)
	// 		if err != nil {
	// 			logger.Fatalf("读取%d 区块出错 err: %v", err)
	// 		}
	// 		if blk == nil {
	// 			logger.Fatalf("读取%d 区块为空 但是block Meta存在")
	// 		}
	// 		node.consensusEngine.ApplyBlock(blk)
	// 	}
	// }

	// 启动P2P
	go node.switcher.Start()
	//// 如果是验证者 启动共识模块
	if node.ws.CurVerfier != nil {
		// 启动共识
		go node.consensusEngine.Daemon()
	} else {
		logger.Info("当前节点不是验证者节点,只能作为普通节点启动...")
	}
	// 启动Blockchain
	go node.chain.Start()
	// 启动交易池
	go node.tx.Start()

	// 启动API服务
	node.apiServer.GET("/", node.apiServer.DefaultHandler)
	node.chain.StartAPI(node.apiServer.Group("/blockchain"))
	node.tx.StartAPI(node.apiServer.Group("/tx"))
	node.consensusEngine.StartAPI(node.apiServer.Group("/consensus"))
	node.ws.StartAPI(node.apiServer.Group("/ws"))
	account.NewAccountApi(node.db).StartAPI(node.apiServer.Group("/account"))
	go node.apiServer.Start()

	sig := make(chan os.Signal)
	signal.Notify(sig, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM,
		syscall.SIGQUIT, syscall.SIGUSR1, syscall.SIGUSR2)

	select {
	case <-sig:
		logger.Infof("signal received...")
		node.consensusEngine.Stop()
		return
	}
}
