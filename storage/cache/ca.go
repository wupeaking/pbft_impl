package cache

import (
	"fmt"
	"path"

	"github.com/golang/protobuf/proto"
	lru "github.com/hashicorp/golang-lru"
	"github.com/wupeaking/pbft_impl/model"
	"github.com/wupeaking/pbft_impl/storage/database"
)

// 增加一个缓存层
type DBCache struct {
	blockDB database.DB
	// 区块缓存
	blocks      *lru.Cache
	blockNum2Id *lru.Cache

	// 账户缓存
	// 交易收据缓存
	// 元数据存储
	accountDB   database.DB
	accountCahe *lru.Cache

	txDB   database.DB
	txCahe *lru.Cache

	txReceiptDB   database.DB
	txReceiptCahe *lru.Cache
	metaDB        database.DB
}

func New(filepath string) *DBCache {

	blockDB, err := database.NewLevelDB(path.Join(filepath, "./pbft/block.db"))
	if err != nil {
		panic(err)
	}
	metaDB, err := database.NewLevelDB(path.Join(filepath, "./pbft/meta.db"))
	if err != nil {
		panic(err)
	}
	txDB, err := database.NewLevelDB(path.Join(filepath, "./pbft/transaction.db"))
	if err != nil {
		panic(err)
	}
	txRecDB, err := database.NewLevelDB(path.Join(filepath, "./pbft/tx_receipts.db"))
	if err != nil {
		panic(err)
	}
	dbCahce := &DBCache{
		blockDB:     blockDB,
		metaDB:      metaDB,
		txDB:        txDB,
		txReceiptDB: txRecDB,
	}
	blkCahce, err := lru.New(1024)
	if err != nil {
		panic(err)
	}
	dbCahce.blocks = blkCahce

	blkNumCache, err := lru.New(10240)
	if err != nil {
		panic(err)
	}
	dbCahce.blockNum2Id = blkNumCache

	accCache, err := lru.New(1024)
	if err != nil {
		panic(err)
	}
	dbCahce.accountCahe = accCache

	txCache, err := lru.New(1024)
	if err != nil {
		panic(err)
	}
	dbCahce.txCahe = txCache

	txRecpt, err := lru.New(10240)
	if err != nil {
		panic(err)
	}
	dbCahce.txReceiptCahe = txRecpt
	return dbCahce
}

func (dbc *DBCache) Insert(value interface{}) error {
	switch x := value.(type) {
	case *model.PbftBlock:
		dbc.blocks.Add(string(x.BlockId), x)
		dbc.blockNum2Id.Add(x.BlockNum, string(x.BlockId))
		v, err := proto.Marshal(x)
		if err != nil {
			return err
		}
		if err := dbc.blockDB.Set(string(x.BlockId), string(v)); err != nil {
			return err
		}
		if err := dbc.blockDB.Set(fmt.Sprintf("%d", x.BlockNum), string(x.BlockId)); err != nil {
			return err
		}

	case *model.BlockMeta:
		v, _ := proto.Marshal(x)
		return dbc.metaDB.Set(string("block_meta"), string(v))

	case *model.Account:
		v, err := proto.Marshal(x)
		if err != nil {
			return err
		}
		dbc.accountCahe.Add(x.Id.GetAddress(), x)
		return dbc.accountDB.Set(x.Id.GetAddress(), string(v))

	case *model.Tx:
		v, err := proto.Marshal(x)
		if err != nil {
			return err
		}
		txID := fmt.Sprintf("%0x", x.Sign)
		dbc.txCahe.Add(txID, x)
		return dbc.txDB.Set(txID, string(v))

	case *model.TxReceipt:
		v, err := proto.Marshal(x)
		if err != nil {
			return err
		}
		id := fmt.Sprintf("%0x", x.Sign)
		dbc.txReceiptCahe.Add(id, x)
		return dbc.txReceiptDB.Set(id, string(v))
	}
	return nil
}

func (dbc *DBCache) GetBlockMeta() (*model.BlockMeta, error) {
	value, err := dbc.metaDB.Get("block_meta")
	if err != nil {
		return nil, err
	}
	if value == "" {
		return nil, nil
	}
	var meta model.BlockMeta
	err = proto.Unmarshal([]byte(value), &meta)
	return &meta, err
}

func (dbc *DBCache) GetBlockByID(id string) (*model.PbftBlock, error) {
	b, ok := dbc.blocks.Get(id)
	if ok {
		return b.(*model.PbftBlock), nil
	}

	value, err := dbc.blockDB.Get(id)
	if err != nil {
		return nil, err
	}
	if value == "" {
		return nil, nil
	}

	var blk model.PbftBlock
	err = proto.Unmarshal([]byte(value), &blk)
	return &blk, err
}

func (dbc *DBCache) GetBlockByNum(num uint64) (*model.PbftBlock, error) {
	bid, ok := dbc.blockNum2Id.Get(num)
	var value string
	if !ok {
		// 先获取blockid
		v, err := dbc.blockDB.Get(fmt.Sprintf("%d", num))
		if err != nil {
			return nil, err
		}
		if v == "" {
			return nil, nil
		}
		value = v
	} else {
		value = bid.(string)
	}

	blkValue, err := dbc.blockDB.Get(value)
	if err != nil {
		return nil, err
	}
	if blkValue == "" {
		return nil, fmt.Errorf("底层数据不一致")
	}

	var blk model.PbftBlock
	err = proto.Unmarshal([]byte(blkValue), &blk)
	return &blk, err
}

func (dbc *DBCache) GetGenesisBlock() (*model.Genesis, error) {
	blk, err := dbc.GetBlockByNum(0)
	if err != nil {
		return nil, err
	}
	if blk == nil {
		return nil, nil
	}

	var g model.Genesis
	g.Verifiers = make([]*model.Verifier, 0)
	for i := range blk.SignPairs {
		g.Verifiers = append(g.Verifiers, &model.Verifier{
			PublickKey: blk.SignPairs[i].SignerId,
		})
	}
	return &g, err
}

func (dbc *DBCache) SetGenesisBlock(genesis *model.Genesis) error {
	blk := model.PbftBlock{
		PrevBlock: model.GenesisPrevBlockId,
		BlockId:   model.GenesisBlockId,
		TimeStamp: uint64(model.GenesisTime),
		BlockNum:  uint64(model.GenesisBlockNum),
		SignPairs: make([]*model.SignPairs, 0),
	}
	for i := range genesis.Verifiers {
		blk.SignPairs = append(blk.SignPairs, &model.SignPairs{
			SignerId: genesis.Verifiers[i].PublickKey,
		})
	}

	v, err := proto.Marshal(&blk)
	if err != nil {
		return err
	}

	if err := dbc.blockDB.Set(fmt.Sprintf("%d", 0), model.GenesisBlockId); err != nil {
		return err
	}

	return dbc.blockDB.Set(model.GenesisBlockId, string(v))
}

func (dbc *DBCache) GetTxByID(id string) (*model.Tx, error) {
	v, ok := dbc.txCahe.Get(id)
	if ok {
		return v.(*model.Tx), nil
	}

	txValue, err := dbc.txDB.Get(id)
	if err != nil {
		return nil, err
	}
	if txValue == "" {
		return nil, fmt.Errorf("底层Tx数据有问题")
	}

	var tx model.Tx
	err = proto.Unmarshal([]byte(txValue), &tx)
	return &tx, err
}

func (dbc *DBCache) GetAccountByID(id string) (*model.Account, error) {
	v, ok := dbc.accountCahe.Get(id)
	if ok {
		return v.(*model.Account), nil
	}

	accValue, err := dbc.accountDB.Get(id)
	if err != nil {
		return nil, err
	}
	if accValue == "" {
		return nil, fmt.Errorf("底层账户数据有问题")
	}

	var acc model.Account
	err = proto.Unmarshal([]byte(accValue), &acc)
	return &acc, err
}

func (dbc *DBCache) GetTxReceiptByID(id string) (*model.TxReceipt, error) {
	v, ok := dbc.txReceiptCahe.Get(id)
	if ok {
		return v.(*model.TxReceipt), nil
	}

	txRValue, err := dbc.txReceiptDB.Get(id)
	if err != nil {
		return nil, err
	}
	if txRValue == "" {
		return nil, fmt.Errorf("底层交易收据数据有问题")
	}

	var txr model.TxReceipt
	err = proto.Unmarshal([]byte(txRValue), &txr)
	return &txr, err
}
