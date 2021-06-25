package transaction

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	log "github.com/sirupsen/logrus"
	"github.com/wupeaking/pbft_impl/common/config"
	"github.com/wupeaking/pbft_impl/model"
	"github.com/wupeaking/pbft_impl/network"
	"github.com/wupeaking/pbft_impl/storage/cache"
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

type txReadMark struct {
	marked bool
}
type TxPool struct {
	switcher network.SwitcherI
	db       *cache.DBCache
	pool     *Pool
	cap      int
	txIds    map[string]txReadMark
	sync.RWMutex
}

func NewTxPool(switcher network.SwitcherI, cfg *config.Configure, db *cache.DBCache) *TxPool {
	switch strings.ToLower(cfg.TxCfg.LogLevel) {
	case "debug":
		logger.Logger.SetLevel(log.DebugLevel)
	case "warn":
		logger.Logger.SetLevel(log.WarnLevel)
	case "info":
		logger.Logger.SetLevel(log.InfoLevel)
	case "error":
		logger.Logger.SetLevel(log.ErrorLevel)
	default:
		logger.Logger.SetLevel(log.InfoLevel)
	}
	pool := NewPool(uint64(cfg.MaxTxNum))

	return &TxPool{
		switcher: switcher,
		pool:     pool,
		cap:      cfg.MaxTxNum,
		txIds:    make(map[string]txReadMark),
		db:       db,
	}
}

func (txpool *TxPool) Start() error {
	if err := txpool.switcher.RegisterOnReceive("transaction", txpool.msgOnRecv); err != nil {
		return err
	}
	return nil
}

func (txpool *TxPool) msgOnRecv(modelID string, msgBytes []byte, p *network.Peer) {
	if modelID != "transaction" {
		return
	}

	var msgPkg network.BroadcastMsg
	if err := json.Unmarshal(msgBytes, &msgPkg); err != nil {
		logger.Debugf("接收的的消息不能被解码 err: %v", err)
		return
	}

	switch msgPkg.MsgType {
	case model.BroadcastMsgType_send_tx:
		// 表示对方发送交易信息
		var txResp model.Txs
		if proto.Unmarshal(msgPkg.Msg, &txResp) != nil {
			return
		}
		//1. 校验交易
		//2. 加入交易池
		needSendtxs := model.Txs{Tansactions: make([]*model.Tx, 0)}
		for _, tx := range txResp.Tansactions {
			if txpool.VerifyTx(tx) != nil {
				continue
			}
			txpool.AddTx(tx)
			needSendtxs.Tansactions = append(needSendtxs.Tansactions, tx)
		}
		if len(needSendtxs.Tansactions) == 0 {
			return
		}
		//  广播给其他节点 但是不广播给接收节点
		msgBody, _ := proto.Marshal(&needSendtxs)
		var broadcastTx network.BroadcastMsg
		broadcastTx.ModelID = "transaction"
		broadcastTx.MsgType = model.BroadcastMsgType_send_tx
		broadcastTx.Msg = msgBody
		txpool.switcher.BroadcastExceptPeer("transaction", &broadcastTx, p)

	default:
		logger.Warnf("transaction 模块不能处理从消息类型")
	}

}

func (txpool *TxPool) GetTx(nums int) []*model.Tx {
	return txpool.pool.scanValue(nums)
}

func (txpool *TxPool) AddTx(tx *model.Tx) bool {
	// todo:: 可能会需要根据cap删除一些
	return txpool.pool.addValue(tx)
}

func (txpool *TxPool) RemoveTx(tx *model.Tx) {
	txpool.pool.delValue(tx)
}

func (txpool *TxPool) VerifyTx(tx *model.Tx) error {
	// 数据格式校验
	// 超过48小时的交易都忽略
	// 或者比当前时间快5分钟
	n := time.Now().Unix()
	if tx.Sender == nil || tx.Sender.Address == "" {
		return fmt.Errorf("交易数据from地址为空")
	}
	if tx.Sequeue == "" {
		return fmt.Errorf("交易数据序列号为空")
	}
	if len(tx.Sign) == 0 {
		return fmt.Errorf("交易数据未签名")
	}
	if len(tx.PublickKey) == 0 {
		return fmt.Errorf("交易数据公钥为空")
	}
	if n-int64(tx.TimeStamp) > 48*3600 || int64(tx.TimeStamp)-n > 5*60 {
		return fmt.Errorf("交易时间戳错误")
	}
	// 首先交易 签名是否正确
	accountAddr := model.PublicKeyToAddress(tx.PublickKey)

	if tx.Sender.Address != accountAddr.Address {
		return fmt.Errorf("账户ID和sender不匹配 id: %s, sender: %s", accountAddr, tx.Sender.Address)
	}
	// 查询账户信息
	account, err := txpool.db.GetAccountByID(tx.Sender.Address)
	if err != nil {
		return err
	}
	if model.Compare(account.Balance.Amount, tx.Amount.Amount) < 0 {
		return fmt.Errorf("余额不足")
	}

	// 签名
	ok, err := tx.VerifySignedTx()
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("验签不通过")
	}
	return nil
}
