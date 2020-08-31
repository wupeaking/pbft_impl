package main

import (
	"crypto/sha256"
	"encoding/json"
	"time"

	"github.com/golang/protobuf/proto"
	log "github.com/sirupsen/logrus"
	"github.com/wupeaking/pbft_impl/common/config"
	"github.com/wupeaking/pbft_impl/consensus"
	cryptogo "github.com/wupeaking/pbft_impl/crypto"
	"github.com/wupeaking/pbft_impl/model"
	"github.com/wupeaking/pbft_impl/network"
	"github.com/wupeaking/pbft_impl/network/http_network"
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
	logger = logg.WithField("module", "coordinator")
}

type Coordinator struct {
	switcher        network.SwitcherI
	cfg             *config.Configure
	requestNewBlock chan struct{}
	maxHeight       *BlockHeight
	pbft            *consensus.PBFT
}

func New(pbft *consensus.PBFT, switcher network.SwitcherI, cfg *config.Configure) *Coordinator {
	return &Coordinator{
		switcher:        switcher,
		cfg:             cfg,
		requestNewBlock: make(chan struct{}, 1),
		maxHeight:       NewBlockHeight(len(cfg.ConsensusCfg.Verfiers)),
		pbft:            pbft,
	}
}

type BlockHeight struct {
	height       uint64
	verifiers    map[string]bool
	verifiersNum int
	trigger      bool
}

func NewBlockHeight(verifiersNum int) *BlockHeight {
	return &BlockHeight{
		height:       0,
		verifiers:    make(map[string]bool),
		verifiersNum: verifiersNum,
	}
}

// 返回当前高度 已经被多少节点认同
func (bh *BlockHeight) UpdateHeight(h uint64, verifier string) bool {
	// 如果新的高度 比当前高 则重置高度
	if h > bh.height {
		bh.height = h
		bh.verifiers = make(map[string]bool)
		bh.trigger = false
		return false

	}
	if h == bh.height {
		bh.verifiers[verifier] = true
		// 要求还未触发 并且有2/3的节点返回
		if !bh.trigger && len(bh.verifiers) >= bh.MinNodeNum() {
			bh.trigger = true
			return true
		}
		return false
	}
	return false
}

func (bh *BlockHeight) MinNodeNum() int {
	f := bh.verifiersNum / 3
	var minNodes int
	if f == 0 {
		minNodes = bh.verifiersNum
	} else {
		minNodes = 2*f + 1
	}
	return minNodes
}

func main() {
	// 尝试读取配置文件
	cfg, err := config.LoadConfig("./.counch/config.json")
	if err != nil {
		logger.Fatalf("读取配置文件发生错误 err: %v", err)
	}
	if len(cfg.ConsensusCfg.Verfiers) == 0 {
		logger.Fatalf("节点数量为空")
	}

	ws := world_state.New("./.counch")
	verifiers := make([]*model.Verifier, 0)
	for i, verfiers := range cfg.Verfiers {
		pub, err := cryptogo.Hex2Bytes(verfiers.Publickey)
		if err != nil {
			logger.Fatalf("验证者公钥格式错误")
		}
		verifiers = append(verifiers, &model.Verifier{PublickKey: pub, SeqNum: int32(i)})
	}
	ws.VerifierNo = -1
	ws.Verifiers = verifiers

	// 启动P2P
	switcher := http_network.New(cfg.NodeAddrs, cfg.LocalAddr, cfg.NetworkCfg.Publickey)
	switcher.Start()
	pbft, err := consensus.New(ws, nil, switcher, cfg)
	if err != nil {
		logger.Fatalf("启动共识引擎出错 err: %v", err)
	}
	coordinator := New(pbft, switcher, cfg)
	switcher.RegisterOnReceive("coordinator", coordinator.msgOnReceive)
	switcher.RegisterOnReceive("blockchain", coordinator.msgOnReceive)
	switcher.RegisterOnReceive("consensus", coordinator.msgOnReceive)

	internalTicker := time.NewTicker(5 * time.Second)

	for {
		select {
		case <-internalTicker.C:
			// 广播 获取最新区块高度
			coordinator.requestBlockHeight()
		case <-coordinator.requestNewBlock:
			logger.Infof("请求新区块")
			coordinator.requestNewBlockProposal()
		}

	}
}

func (cd *Coordinator) msgOnReceive(modelID string, msgBytes []byte, p *network.Peer) {
	var msgPkg network.BroadcastMsg
	if json.Unmarshal(msgBytes, &msgPkg) != nil {
		return
	}

	switch msgPkg.MsgType {
	case model.BroadcastMsgType_send_specific_block:
		// 表示对方向本节点发送区块信息
		var blockResp model.BlockResponse
		if proto.Unmarshal(msgPkg.Msg, &blockResp) != nil {
			return
		}
		if blockResp.RequestType == model.BlockRequestType_only_header {
			// 校验区块头
			if !cd.pbft.VerfifyBlockHeader(blockResp.Block) {
				return
			}
			pub, _ := cryptogo.Hex2Bytes(p.ID)
			cd.pbft.IsVaildVerifier(pub)
			if cd.maxHeight.UpdateHeight(blockResp.Block.BlockNum, p.ID) {
				// 发起新区块提议
				// todo::: 暂时还没考虑当某个主节点down机 超时之后怎么办
				select {
				case cd.requestNewBlock <- struct{}{}:
				default:
				}
			}

		}
	}
}

func (cd *Coordinator) requestBlockHeight() {
	request := model.BlockRequest{
		RequestType: model.BlockRequestType_only_header,
		BlockNum:    -1,
	}
	body, _ := proto.Marshal(&request)
	msg := network.BroadcastMsg{
		// 发给对方节点的 Blockchain模块
		ModelID: "blockchain",
		MsgType: model.BroadcastMsgType_request_load_block,
		Msg:     body,
	}
	cd.switcher.Broadcast(msg.ModelID, &msg)
}

func (cd *Coordinator) requestNewBlockProposal() {
	msgInfo := &model.PbftMessageInfo{
		MsgType: model.MessageType_NewBlockProposal,
		SeqNum:  0,
		View:    0,
	}
	content, _ := proto.Marshal(msgInfo)
	sh := sha256.New()
	sh.Write(content)
	hash := sh.Sum(nil)
	privKey, err := cryptogo.LoadPrivateKey(cd.cfg.Coordinator.PriVateKey)
	if err != nil {
		logger.Errorf("加载协调者私钥失败, err: %v", err)
		return
	}
	sign, err := cryptogo.Sign(privKey, hash)
	if err != nil {
		logger.Errorf("签名消息失败, err: %v", err)
		return
	}
	s, _ := cryptogo.Hex2Bytes(sign)
	msgInfo.Sign = s

	request := model.PbftGenericMessage{
		Info: msgInfo,
	}
	body, _ := proto.Marshal(&request)
	msg := network.BroadcastMsg{
		// 发给对方节点的 consensus模块
		ModelID: "consensus",
		MsgType: model.BroadcastMsgType_send_pbft_msg,
		Msg:     body,
	}
	cd.switcher.Broadcast(msg.ModelID, &msg)
}
