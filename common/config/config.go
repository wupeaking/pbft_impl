package config

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
	cryptogo "github.com/wupeaking/pbft_impl/crypto"
)

// var logger *log.Logger
var logger *log.Entry

func init() {
	logg := log.New()
	logg.SetLevel(log.DebugLevel)
	logg.SetReportCaller(true)
	logg.SetFormatter(&log.TextFormatter{
		DisableColors: true,
		FullTimestamp: true,
	})
	logger = logg.WithField("module", "node")

	// logger.Logger.SetFormatter(&log.TextFormatter{
	// 	DisableColors: true,
	// 	FullTimestamp: true,
	// })
}

type ConsensusCfg struct {
	Publickey  string `json:"publicKey" yaml:"publicKey"`
	PriVateKey string `json:"privateKey" yaml:"privateKey"`
	Verfiers   []struct {
		Publickey  string `json:"publicKey" yaml:"publicKey"`
		PriVateKey string `json:"privateKey" yaml:"privateKey"`
	} `json:"verfiers" yaml:"verfiers"`
	Timeout     int `json:"timeout" yaml:"timeout"`
	Coordinator struct {
		Publickey  string `json:"publicKey" yaml:"publicKey"`
		PriVateKey string `json:"privateKey" yaml:"privateKey"`
	} `json:"coordinator"`
	LogLevel string `json:"logLevel"`
}

type TxCfg struct {
	MaxTxNum int    `json:"maxTxNum" yaml:"maxTxNum"` // 本地最大交易池数量
	LogLevel string `json:"logLevel"`
}

type NetworkCfg struct {
	NetMode        string     `json:"netMode" yaml:"netMode"`       // 暂时只支持http模式
	LocalAddr      string     `json:"localAddr" yaml:"localAddr"`   // 本机地址
	NodeAddrs      []NodeAddr `json:"nodeAddrs" yaml:"nodeAddrs"`   // 节点地址列表
	Publickey      string     `json:"publicKey" yaml:"publicKey"`   // 节点ID
	PriVateKey     string     `json:"privateKey" yaml:"privateKey"` // 节点私钥
	LogLevel       string     `json:"logLevel"`
	Bootstrap      bool       `json:"bootstrap"`
	BootstrapPeers []string   `json:"bootstrapPeers"`
}

type NodeAddr struct {
	Address string `json:"address"`
	PeerID  string `json:"peerID"`
}

type DBCfg struct {
	StorageEngine string `json:"storageEngine" yaml:"storageEngine"` // 暂时只有leveldb
	LogLevel      string `json:"logLevel"`
}

type WebCfg struct {
	Port int `json:"port" yaml:"port"`
}

type Account struct {
	Address string `json:"address" yaml:"address"`
	Type    int    `json:"type"`
	Amount  uint64 `json:"amount"`
}

type AccountCfg []Account

type Configure struct {
	ConsensusCfg `json:"consensus"`
	TxCfg        `json:"transaction"`
	NetworkCfg   `json:"network"`
	DBCfg        `json:"db"`
	WebCfg       `json:"web"`
	AccountCfg   `json:"account"`
}

// 加载和初始化配置
func LoadConfig(file string) (*Configure, error) {
	if !fileExist(file) {
		logger.Warnf("未读取到配置文件 使用默认配置")
		def, _ := DefaultConfig()
		cfgValue, err := json.Marshal(def)
		if err != nil {
			return def, err
		}
		fd, err := os.Create(file)
		if err != nil {
			return def, err
		}
		defer fd.Close()
		_, err = io.Copy(fd, strings.NewReader(string(cfgValue)))
		return def, err
	}
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	content, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}
	var cfg Configure
	err = json.Unmarshal(content, &cfg)
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

// DefaultConfig 生成一个默认配置
func DefaultConfig() (*Configure, error) {
	// 生成一个随机的公私钥对
	priv, pub, err := cryptogo.GenerateKeyPairs()
	if err != nil {
		return nil, err
	}
	return &Configure{
		ConsensusCfg{
			PriVateKey: priv,
			Publickey:  pub,
			Verfiers: []struct {
				Publickey  string `json:"publicKey" yaml:"publicKey"`
				PriVateKey string `json:"privateKey" yaml:"privateKey"`
			}{
				{
					Publickey: "0xc4024ffd0b42495f49002b5da606512aee341c53e43a641b7d8efac8e29f6ed2d5c6449fe4343f41c5216a84ea9dd43e07daeeadb38556bb19527ce699394cd7",
				},
				{
					Publickey: "0x302404eeb2e3d1e75f78f426836cb6ee741d735153e441f1f43fbec55b4482c6d2d59017e608b995ba32255b31c49b646d59834537b9c2efb7cd66c64250c5b2",
				},
				{
					Publickey: "0x5ca153355f800c66150130b8becb951856e408555829eb07de89d3ed35fdd85872923fd9c51444ace5df3d6ce331da676a5e90596e7952f3f4a05c623bc00d77",
				},
			},
			Timeout:  10,
			LogLevel: "info",
		},
		TxCfg{
			MaxTxNum: 10000,
			LogLevel: "info",
		},
		NetworkCfg{
			NetMode:    "p2p",
			LocalAddr:  "0.0.0.0:19876",
			Publickey:  pub,
			PriVateKey: priv,
			LogLevel:   "info",
			Bootstrap:  false,
		},
		DBCfg{
			StorageEngine: "levelDB",
		},
		WebCfg{
			Port: 8088,
		},
		AccountCfg{
			Account{
				Address: "0xf52772d71e21a42e8cd2c5987ed3bb99420fecf4c7aca797b926a8f01ea6ffd8",
				Amount:  100000000,
				Type:    1,
			},
			Account{
				Address: "0x4abdf9c6391f193f3829f25204a1309d5c1c53dceb920c529c65a04d3d6c7317",
				Amount:  100000000,
				Type:    1,
			},
			Account{
				Address: "0xf5e99aea09cf53e2e19c3d10b1d6283bfa1ab67c65d9fb29c75122561705bf56",
				Amount:  100000000,
				Type:    1,
			},
		},
	}, nil
}

func fileExist(file string) bool {
	_, err := os.Stat(file) //os.Stat获取文件信息
	if err != nil {
		return os.IsExist(err)
	}
	return true
}
