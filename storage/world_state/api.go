package world_state

import (
	"encoding/json"

	"github.com/labstack/echo"
	"github.com/wupeaking/pbft_impl/api"
)

func (ws *WroldState) StartAPI(g *echo.Group) {
	g.GET("/", ws.rootHandler)
	g.GET("/status", ws.statusHandler)
	g.GET("/last_txs", ws.lastTxsHandler)
	g.GET("/last_blocks", ws.lastBlocksHandler)
}

func (ws *WroldState) rootHandler(ctx echo.Context) error {
	return ctx.Blob(200, "application/json", []byte(`
	GET /ws/status   当前区块状态
	GET /ws/last_txs   最近的10条交易
	`))
}

func (ws *WroldState) statusHandler(ctx echo.Context) error {
	resp := struct {
		TxNum      uint64 `json:"tx_num"`
		BlockNum   uint64 `json:"block_num"`
		LastView   uint64 `json:"last_view"`
		VerfierNum int    `json:"verfier_num"`
	}{}
	countSQL := `select count(*) from records ;`
	var count uint64
	ws.txRecordDB.QueryRowx(countSQL).Scan(&count)
	resp.TxNum = count
	resp.BlockNum = ws.BlockNum
	resp.LastView = ws.View
	resp.VerfierNum = len(ws.Verifiers)
	respBody, _ := json.Marshal(resp)

	return ctx.Blob(200, "application/json", respBody)
}

func (ws *WroldState) lastTxsHandler(ctx echo.Context) error {
	type txInfo struct {
		TxID     string `json:"tx_id"`
		From     string `json:"from"`
		To       string `json:"to"`
		Statue   int    `json:"status"`
		Amount   string `json:"amount"`
		BlockNum uint64 `json:"block_num"`
	}
	sql := `select tx_id, sender, reciept, amount, status, block_num from records order by id desc limit 10;`
	rows, err := ws.txRecordDB.Queryx(sql)
	if err != nil {
		return &echo.HTTPError{Code: -1, Internal: err}
	}
	resp := make([]txInfo, 0)
	for rows.Next() {
		var t txInfo
		rows.Scan(&t.TxID, &t.From, &t.To, &t.Amount, &t.Statue, &t.BlockNum)
		resp = append(resp, t)
	}
	return api.DataPackage(0, "success", resp, ctx)
}

func (ws *WroldState) lastBlocksHandler(ctx echo.Context) error {
	type blkInfo struct {
		ID       string `json:"id"`
		BlockNum uint64 `json:"block_num"`
		TxNums   int    `json:"tx_num"`
	}
	curNum := ws.BlockNum

	resp := make([]blkInfo, 0)
	for i := curNum; i > curNum-10 && i > 0; i-- {
		var v blkInfo
		blk, _ := ws.db.GetBlockByNum(i)
		if blk != nil {
			v.ID = blk.BlockId
			v.BlockNum = blk.BlockNum
			v.TxNums = len(blk.Tansactions.Tansactions)
		}
		resp = append(resp, v)
	}
	return api.DataPackage(0, "success", resp, ctx)
}
