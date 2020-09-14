package libp2p

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	discovery "github.com/libp2p/go-libp2p-discovery"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	ma "github.com/multiformats/go-multiaddr"
	log "github.com/sirupsen/logrus"
	"github.com/wupeaking/pbft_impl/common/config"
	cryptogo "github.com/wupeaking/pbft_impl/crypto"
)

// 使用开源的libp2p实现P2P组件

var logger *log.Entry
var defaultBootstraps []string

func init() {
	logg := log.New()
	logg.SetLevel(log.DebugLevel)
	logg.SetReportCaller(true)
	logg.SetFormatter(&log.TextFormatter{
		DisableColors: true,
		FullTimestamp: true,
	})
	logger = logg.WithField("module", "P2P")
}

type P2PNetWork struct {
	Host           host.Host
	protocol       string
	rendezvous     string
	boostrapPeers  []ma.Multiaddr
	bootstarp      bool
	kademliaDHT    *dht.IpfsDHT
	routeDiscovery *discovery.RoutingDiscovery
	sync.RWMutex
	books map[string]network.Stream
}

func New(cfg *config.Configure) (*P2PNetWork, error) {
	// 加载私钥
	priv, err := cryptogo.LoadPrivateKey(cfg.NetworkCfg.PriVateKey)
	if err != nil {
		return nil, err
	}
	pri, _, err := crypto.ECDSAKeyPairFromKey(priv)
	if err != nil {
		return nil, err
	}
	// 解析监听地址
	listen := strings.Split(cfg.NetworkCfg.LocalAddr, ":")
	if len(listen) != 2 {
		return nil, fmt.Errorf("监听地址格式错误")
	}

	ctx := context.Background()
	host, err := libp2p.New(
		ctx,
		libp2p.Identity(pri),
		libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/%s/tcp/%s", listen[0], listen[1])),
	)
	if err != nil {
		return nil, err
	}

	p2p := &P2PNetWork{
		Host:       host,
		protocol:   "/counch/1.0.0",
		rendezvous: "counch-p2p-discover",
	}
	p2p.bootstarp = cfg.NetworkCfg.Bootstrap
	bootstraps := cfg.NetworkCfg.BootstrapPeers
	if len(bootstraps) == 0 {
		bootstraps = defaultBootstraps
	}
	for _, peerAddr := range bootstraps {
		mAddr, err := ma.NewMultiaddr(peerAddr)
		if err != nil {
			return nil, err
		}
		p2p.boostrapPeers = append(p2p.boostrapPeers, mAddr)
	}

	dhtOps := []dht.Option{}
	if p2p.bootstarp {
		dhtOps = append(dhtOps, dht.Mode(dht.ModeServer))
	}
	kademliaDHT, err := dht.New(ctx, p2p.Host, dhtOps...)
	if err != nil {
		return nil, err
	}
	p2p.kademliaDHT = kademliaDHT
	// 创建路由表
	routingDiscovery := discovery.NewRoutingDiscovery(kademliaDHT)
	p2p.routeDiscovery = routingDiscovery
	p2p.books = make(map[string]network.Stream)

	return p2p, nil
}

func (p2p *P2PNetWork) Start() error {
	logger.Debugf("启动P2P模块, ID: %s", p2p.Host.ID())
	p2p.Host.SetStreamHandler(protocol.ID(p2p.protocol), p2p.streamHandler)

	ctx := context.Background()
	// 启动分布式hash表
	if err := p2p.kademliaDHT.Bootstrap(ctx); err != nil {
		return err
	}
	if p2p.bootstarp {
		return nil
	}

	// 连接到启动节点 广播自己的位置
	for _, addr := range p2p.boostrapPeers {
		peerinfo, err := peer.AddrInfoFromP2pAddr(addr)
		if err != nil {
			return err
		}
		go func(pi *peer.AddrInfo) {
			if err := p2p.Host.Connect(ctx, *pi); err != nil {
				logger.Infof("连接公共启动节点失败: %s\n", err.Error())
			} else {
				logger.Infof("连接公共启动节点成功: peerinfo: %v\n", pi)
			}
		}(peerinfo)
	}

	discovery.Advertise(ctx, p2p.routeDiscovery, p2p.rendezvous)
	go p2p.NodeDiscovery()
	return nil
}

func (p2p *P2PNetWork) NodeDiscovery() {
	peerChan, err := p2p.routeDiscovery.FindPeers(context.Background(), p2p.rendezvous)
	if err != nil {
		logger.Errorf("节点发现出现异常, err: %s", err.Error())
	}
	for peer := range peerChan {
		if peer.ID == p2p.Host.ID() {
			logger.Debugf("搜索的节点是自己 忽略, peer id: %s\n", peer.ID)
			continue
		}
		if _, ok := p2p.books[peer.ID.String()]; ok {
			logger.Debugf("此peer已经连接 %s\n", peer.ID.String())
			continue
		}
		stream, err := p2p.Host.NewStream(context.Background(), peer.ID, protocol.ID(p2p.protocol))
		if err != nil {
			logger.Infof("p2p Connection failed: %v\n", err)
			continue
		} else {
			p2p.Lock()
			p2p.books[peer.ID.String()] = stream
			p2p.Unlock()
			// 启动goroutine 处理数据
		}
	}
}

func (p2p *P2PNetWork) streamHandler(stream network.Stream) {
	p2p.Lock()
	p2p.books[stream.ID()] = stream
	p2p.Unlock()

	go p2p.dataStreamRecv(stream)
}
