package transaction

import (
	"encoding/json"

	"github.com/golang/protobuf/proto"
	log "github.com/sirupsen/logrus"
	"github.com/wupeaking/pbft_impl/model"
	"github.com/wupeaking/pbft_impl/network"
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

type TxPool struct {
	switcher network.SwitcherI
}

func NewTxPool(switcher network.SwitcherI) *TxPool {
	return &TxPool{
		switcher: switcher,
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
		//  广播给其他节点
		msgBody, _ := proto.Marshal(&needSendtxs)
		var broadcastTx network.BroadcastMsg
		broadcastTx.ModelID = "transaction"
		broadcastTx.MsgType = model.BroadcastMsgType_send_tx
		broadcastTx.Msg = msgBody
		txpool.switcher.Broadcast("transaction", &broadcastTx)

	default:
		logger.Warnf("transaction 模块不能处理从消息类型")
	}

}

func (txpool *TxPool) GetTx() *model.Tx {
	return nil
}

func (txpool *TxPool) AddTx(*model.Tx) bool {
	return true
}

func (txpool *TxPool) RemoveTx(*model.Tx) {

}

func (txpool *TxPool) VerifyTx(*model.Tx) error {
	return nil
}
