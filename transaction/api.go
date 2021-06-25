package transaction

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/labstack/echo"
	"github.com/wupeaking/pbft_impl/api"
	cryptogo "github.com/wupeaking/pbft_impl/crypto"
	"github.com/wupeaking/pbft_impl/model"
)

func (t *TxPool) StartAPI(g *echo.Group) {
	g.GET("/", t.rootHandler)
	g.GET("/tansaction/status", t.statusHandler)
	g.PUT("/tansaction/:txid", t.addTxHandler)
	g.GET("/tansaction/:txid", t.queryTxHandler)
}

func (t *TxPool) rootHandler(ctx echo.Context) error {
	return ctx.Blob(200, "application/json", []byte(`
	GET /tx/tansaction/status   当前交易池状态
	PUT /tx/tansaction/:txid  发起一个新的交易
	GET /tx/tansaction/:txid  查询交易信息
	`))
}

func (t *TxPool) statusHandler(ctx echo.Context) error {
	resp := struct {
		PoolUsed int `json:"pool_used"`
		PoolSize int `json:"pool_size"`
	}{PoolSize: t.cap, PoolUsed: int(t.pool.len())}
	respBody, _ := json.Marshal(resp)

	return ctx.Blob(200, "application/json", respBody)
}

func (t *TxPool) addTxHandler(ctx echo.Context) error {
	request := struct {
		From      string `json:"from"`
		To        string `json:"to"`
		Amount    uint64 `json:"amount"`
		Sign      string `json:"sign"`
		PublicKey string `json:"publick_key"`
		Sequeue   string `json:"sequeue"`
		Timestamp uint64 `json:"timestamp"`
	}{}

	content, err := ioutil.ReadAll(ctx.Request().Body)
	if err != nil {
		return &echo.HTTPError{Code: -1, Internal: err}
	}
	if err := json.Unmarshal(content, &request); err != nil {
		return &echo.HTTPError{Code: -1, Internal: err}
	}
	signBytes, err := cryptogo.Hex2Bytes(request.Sign)
	if err != nil {
		return &echo.HTTPError{Code: -1, Internal: err}
	}
	pub, err := cryptogo.Hex2Bytes(request.PublicKey)
	if err != nil {
		return &echo.HTTPError{Code: -1, Internal: err}
	}
	fmt.Printf("received tx %#v\n", request)

	tx := &model.Tx{
		Sender:     &model.Address{Address: request.From},
		Recipient:  &model.Address{Address: request.To},
		Sign:       signBytes,
		PublickKey: pub,
		TimeStamp:  request.Timestamp,
		Sequeue:    request.Sequeue,
		Amount:     &model.Amount{Amount: fmt.Sprintf("%d", request.Amount)},
	}
	if err := t.VerifyTx(tx); err != nil {
		return &echo.HTTPError{Code: -1, Internal: err}
	}
	if !t.AddTx(tx) {
		return &echo.HTTPError{Code: -1, Internal: fmt.Errorf("交易池已满")}
	}
	return api.DataPackage(0, "success", nil, ctx)
}

func (t *TxPool) queryTxHandler(ctx echo.Context) error {
	id := ctx.Param("txid")
	tx, err := t.db.GetTxByID(id)
	if err != nil {
		return &echo.HTTPError{Code: -1, Internal: err}
	}
	if tx == nil {
		return &echo.HTTPError{Code: http.StatusNotFound, Internal: fmt.Errorf("未查询到此交易")}
	}

	return api.DataPackage(0, "success", tx, ctx)
}
