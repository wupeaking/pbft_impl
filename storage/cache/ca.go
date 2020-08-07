package cache

import (
	"fmt"

	"github.com/golang/protobuf/proto"
	"github.com/wupeaking/pbft_impl/model"
	"github.com/wupeaking/pbft_impl/storage/database"
)

// 增加一个缓存层
type DBCache struct {
	blockDB database.DB
	// 区块缓存
	blocks     map[string]*model.PbftBlock
	blocknumId map[uint64]string

	// 账户缓存
	// 交易收据缓存
	// 元数据存储
	metaDB database.DB
}

func New(path string) *DBCache {
	blockDB, err := database.NewLevelDB(path + "./pbft/block.db")
	if err != nil {
		panic(err)
	}
	metaDB, err := database.NewLevelDB(path + "./pbft/meta.db")
	if err != nil {
		panic(err)
	}
	return &DBCache{
		blockDB:    blockDB,
		blocks:     make(map[string]*model.PbftBlock),
		blocknumId: make(map[uint64]string),
		metaDB:     metaDB,
	}
}

func (dbc *DBCache) Insert(value interface{}) error {
	switch x := value.(type) {
	case *model.PbftBlock:
		dbc.blocks[string(x.BlockId)] = x
		dbc.blocknumId[x.BlockNum] = string(x.BlockId)
		v, _ := proto.Marshal(x)
		if err := dbc.blockDB.Set(string(x.BlockId), string(v)); err != nil {
			return err
		}
		if err := dbc.blockDB.Set(fmt.Sprintf("%d", x.BlockNum), string(x.BlockId)); err != nil {
			return err
		}

	case *model.BlockMeta:
		v, _ := proto.Marshal(x)
		return dbc.blockDB.Set(string("block_meta"), string(v))
	}
	return nil
}

func (dbc *DBCache) GetBlockByID(id string) (*model.PbftBlock, error) {
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
	value, err := dbc.blockDB.Get(fmt.Sprintf("%d", num))
	if err != nil {
		return nil, err
	}
	if value == "" {
		return nil, nil
	}

	value, err = dbc.blockDB.Get(value)
	if err != nil {
		return nil, err
	}
	if value == "" {
		return nil, fmt.Errorf("底层数据不一致")
	}

	var blk model.PbftBlock
	err = proto.Unmarshal([]byte(value), &blk)
	return &blk, err
}

func (dbc *DBCache) GetGenesisBlock() (*model.Genesis, error) {
	value, err := dbc.blockDB.Get(fmt.Sprintf("%d", 0))
	if err != nil {
		return nil, err
	}
	if value == "" {
		return nil, nil
	}

	var blk model.Genesis
	err = proto.Unmarshal([]byte(value), &blk)
	return &blk, err
}
