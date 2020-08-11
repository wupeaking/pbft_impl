package database

import (
	"github.com/syndtr/goleveldb/leveldb"
)

type LevelDB struct {
	*leveldb.DB
}

func NewLevelDB(path string) (DB, error) {
	db, err := leveldb.OpenFile(path, nil)
	return &LevelDB{db}, err
}

func (ldb *LevelDB) Get(key string) (string, error) {
	v, err := ldb.DB.Get([]byte(key), nil)
	if err == leveldb.ErrNotFound {
		return "", nil
	}
	return string(v), err
}

func (ldb *LevelDB) Set(key, value string) error {
	return ldb.Put([]byte(key), []byte(value), nil)
}

func (ldb *LevelDB) Delete(key string) error {
	return ldb.DB.Delete([]byte(key), nil)
}
