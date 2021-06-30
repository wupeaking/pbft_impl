package world_state

import (
	"fmt"
	"strings"
	"sync"

	"github.com/jmoiron/sqlx"
	"github.com/wupeaking/pbft_impl/common"
	"github.com/wupeaking/pbft_impl/model"
	"github.com/wupeaking/pbft_impl/storage/cache"
)

// 定义整个全局状态

type WroldState struct {
	BlockNum     uint64            `json:"blockNum"`  // 当前区块编号
	PrevBlock    string            `json:"prevBlock"` // 前一个区块hash
	BlockID      string            `json:"blockID"`   // 当前区块hash
	Verifiers    []*model.Verifier `json:"verifiers"` // 当前所有的验证者
	VerifiersMap map[string]struct{}
	VerifierNo   int             `josn:"verifierNo"` // 验证者所处编号 如果为-1  表示不是验证者
	CurVerfier   *model.Verifier `json:"curVerfier"`
	View         uint64          `json:"view"` // 当前视图
	db           *cache.DBCache
	sync.RWMutex `json:"-"`
	txRecordDB   *sqlx.DB
}

func New(dbCache *cache.DBCache, txRecordPath string) *WroldState {
	if txRecordPath == "" {
		return &WroldState{db: dbCache}
	}
	// 判断文件是否存在 如果不存在则创建
	if !common.FileExist(txRecordPath + "/tx_records.db") {
		db, err := sqlx.Open("sqlite3", txRecordPath+"/tx_records.db")
		if err != nil {
			panic(err)
		}
		var schema = `
		CREATE TABLE if not exists records (
			id    integer PRIMARY KEY autoincrement,
			tx_id VARCHAR(512)  DEFAULT '',
			sender  VARCHAR(256)  DEFAULT '',
			reciept VARCHAR(256) DEFAULT '',
			amount VARCHAR(256) DEFAULT '',
			tx_reciept VARCHAR(256) DEFAULT '',
			status integer,
			block_num integer
		);
		`
		_, err = db.Exec(schema)
		if err != nil {
			panic(err)
		}
		indexSql := `CREATE INDEX tx_id_index
		on records (tx_id);
		`
		_, err = db.Exec(indexSql)
		if err != nil {
			panic(err)
		}

		return &WroldState{db: dbCache, txRecordDB: db}
	} else {
		db, err := sqlx.Open("sqlite3", txRecordPath+"/tx_records.db")
		if err != nil {
			panic(err)
		}
		return &WroldState{db: dbCache, txRecordDB: db}
	}
}

func (ws *WroldState) IncreaseBlockNum() {
	ws.Lock()
	ws.BlockNum++
	ws.Unlock()
}

func (ws *WroldState) IncreaseView() {
	ws.Lock()
	ws.View++
	ws.Unlock()
}

func (ws *WroldState) SetBlockNum(num uint64) {
	ws.Lock()
	defer func() { ws.Unlock() }()
	ws.BlockNum = num
}

func (ws *WroldState) SetView(v uint64) {
	ws.Lock()
	defer func() { ws.Unlock() }()
	ws.View = v
}

func (ws *WroldState) SetValue(blockNum uint64, prevBlock string, blockID string,
	verifiers []*model.Verifier) {
	ws.Lock()
	defer func() { ws.Unlock() }()
	if blockNum != 0 {
		ws.BlockNum = blockNum
	}
	if prevBlock != "" {
		ws.PrevBlock = prevBlock
	}
	if blockID != "" {
		ws.BlockID = blockID
	}
	if len(verifiers) > 0 {
		ws.Verifiers = verifiers
	}
}

func (ws *WroldState) IsVerfier(publicKey []byte) bool {
	ws.RLock()
	defer func() { ws.RUnlock() }()
	_, ok := ws.VerifiersMap[string(publicKey)]
	return ok
}

func (ws *WroldState) updateVerifierMap() {
	ws.Lock()
	defer func() { ws.Unlock() }()
	newValue := make(map[string]struct{})
	for i := range ws.Verifiers {
		newValue[string(ws.Verifiers[i].PublickKey)] = struct{}{}
	}
	ws.VerifiersMap = newValue
}

func (ws *WroldState) InsertTxRecords(txs []*model.Tx, txrs []*model.TxReceipt, blockNum int64) error {
	if len(txs) == 0 || len(txrs) == 0 || len(txs) != len(txrs) {
		return nil
	}
	if ws.txRecordDB == nil {
		return nil
	}

	holder := make([]string, 0, len(txs))
	values := make([]interface{}, 0, len(txs)*7)
	for i := range txs {
		h := []string{
			fmt.Sprintf("$%d", i*7+1), fmt.Sprintf("$%d", i*7+2),
			fmt.Sprintf("$%d", i*7+3), fmt.Sprintf("$%d", i*7+4),
			fmt.Sprintf("$%d", i*7+5), fmt.Sprintf("$%d", i*7+6),
			fmt.Sprintf("$%d", i*7+7),
		}
		holder = append(holder, fmt.Sprintf("(%s)", strings.Join(h, ",")))
		values = append(values, fmt.Sprintf("%0x", txs[i].Sign), txs[i].Sender.Address,
			txs[i].Recipient.Address, txs[i].Amount.Amount,
			fmt.Sprintf("%0x", txrs[i].Sign), txrs[i].Status, blockNum)
	}
	smt := fmt.Sprintf(`insert into records(tx_id, sender, reciept, amount, tx_reciept, status, block_num) values %s`,
		strings.Join(holder, ", "))
	_, err := ws.txRecordDB.Exec(smt, values...)
	return err
}
