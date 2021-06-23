package blockchain

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/labstack/echo"
	"github.com/wupeaking/pbft_impl/api"
)

func (bc *BlockChain) StartAPI(g *echo.Group) {
	g.GET("/", bc.rootHandler)
	g.GET("/status", bc.statusHandler)
	g.GET("/block/:num", bc.queryBlockHandler)
}

func (bc *BlockChain) rootHandler(ctx echo.Context) error {
	return ctx.Blob(200, "application/json", []byte(`
	GET /blockchain/status   当前区块状态
	GET /blockchain/block/:num  查询某个区块的信息
	`))
}

func (bc *BlockChain) statusHandler(ctx echo.Context) error {
	resp := struct {
		CurBlockHeight uint64 `json:"cur_block_height"`
		MaxBlockHeight uint64 `json:"max_block_height"`
	}{CurBlockHeight: bc.ws.BlockNum, MaxBlockHeight: bc.pool.maxHeight}

	respBody, _ := json.Marshal(resp)

	return ctx.Blob(200, "application/json", respBody)
}

func (bc *BlockChain) queryBlockHandler(ctx echo.Context) error {
	num := ctx.Param("num")
	blockNum, _ := strconv.ParseUint(num, 10, 64)
	if blockNum == 0 {
		blockNum = bc.ws.BlockNum
	}
	blk, err := bc.ws.GetBlock(blockNum)
	if err != nil {
		return &echo.HTTPError{Code: -1, Internal: err}
	}
	if blk == nil {
		return &echo.HTTPError{Code: http.StatusNotFound, Internal: fmt.Errorf("未查询到此区块")}
	}

	return api.DataPackage(0, "success", blk, ctx)
}
