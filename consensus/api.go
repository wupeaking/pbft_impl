package consensus

import (
	"encoding/json"

	"github.com/labstack/echo"
	"github.com/wupeaking/pbft_impl/model"
)

func (pbft *PBFT) StartAPI(g *echo.Group) {
	g.GET("/", pbft.rootHandler)
	g.GET("/status", pbft.statusHandler)
}

func (pbft *PBFT) rootHandler(ctx echo.Context) error {
	return ctx.Blob(200, "application/json", []byte(`
	GET /consensus/status   当前共识状态
	`))
}

func (pbft *PBFT) statusHandler(ctx echo.Context) error {
	resp := struct {
		Status    string `json:"consensus_status"`
		IsVerfier bool   `json:"is_verfier"`
		No        int    `json:"no"`
		BlockNum  int64  `json:"block_num"`
		View      int64  `json:"view"`
	}{
		Status:    model.States_name[int32(pbft.CurrentState())],
		IsVerfier: pbft.ws.CurVerfier != nil,
		No:        pbft.ws.VerifierNo,
		BlockNum:  int64(pbft.ws.BlockNum),
		View:      int64(pbft.ws.View),
	}
	respBody, _ := json.Marshal(resp)
	return ctx.Blob(200, "application/json", respBody)
}
