package cache

import (
	"github.com/golang/protobuf/proto"
	"github.com/wupeaking/pbft_impl/model"
	"github.com/wupeaking/pbft_impl/storage/database"
)

// 增加一个缓存层
type DBCache struct {
	blockDB database.DB
	// 区块缓存
	blocks map[string]*model.PbftBlock
	// 账户缓存
	// 交易收据缓存
}

func New() *DBCache {
	blockDB, err := database.NewLevelDB("./pbft/block.db")
	if err != nil {
		panic(err)
	}
	return &DBCache{
		blockDB: blockDB,
		blocks:  make(map[string]*model.PbftBlock),
	}
}

func (dbc *DBCache) Insert(value interface{}) error {
	switch x := value.(type) {
	case *model.PbftBlock:
		dbc.blocks[string(x.BlockId)] = x
		v, _ := proto.Marshal(x)
		return dbc.blockDB.Set(string(x.BlockId), string(v))
	}
	return nil
}
