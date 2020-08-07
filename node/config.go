package node

type ConsensusCfg struct {
	Publickey  string `json:"publicKey" yaml:"publicKey"`
	PriVateKey string `json:"privateKey" yaml:"privateKey"`
	Verfiers   []struct {
		Publickey  string `json:"publicKey" yaml:"publicKey"`
		PriVateKey string `json:"privateKey" yaml:"privateKey"`
	} `json:"verfiers" yaml:"verfiers"`
	Timeout int `json:"timeout" yaml:"timeout"`
}

type TxCfg struct {
	MaxTxNum int `json:"maxTxNum" yaml:"maxTxNum"` // 本地最大交易池数量
}

type NetworkCfg struct {
	NetMode   string   `json:"netMode" yaml:"netMode"`     // 暂时只支持http模式
	LocalAddr string   `json:"localAddr" yaml:"localAddr"` // 本机地址
	NodeAddrs []string `json:"nodeAddrs" yaml:"nodeAddrs"` // 节点地址列表
}

type DBCfg struct {
	StorageEngine string `json:"storageEngine" yaml:"storageEngine"` // 暂时只有leveldb
}

type Configure struct {
	ConsensusCfg `json:"consensus"`
	TxCfg        `json:"transaction"`
	NetworkCfg   `json:"network"`
	DBCfg        `json:"db"`
}

// 加载和初始化配置
func LoadConfig(file string) *Configure {
	return nil
}
