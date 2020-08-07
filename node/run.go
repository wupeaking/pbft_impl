package node

import (
	"github.com/wupeaking/pbft_impl/consensus"
	"github.com/wupeaking/pbft_impl/network"
	"github.com/wupeaking/pbft_impl/storage/world_state"
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

	if genesis == nil {
		// cfg.ConsensusCfg.Publickey
		// cfg.ConsensusCfg.PriVateKey
	} else {
		// 验证配置文件是否和Genesis有冲突
	}
	//http_network.New(nodeAddrs []string, local string)
	return nil
}
