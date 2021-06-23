package account

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo"
	"github.com/wupeaking/pbft_impl/api"
	"github.com/wupeaking/pbft_impl/storage/cache"
)

type AccountApi struct {
	db *cache.DBCache
}

func NewAccountApi(db *cache.DBCache) *AccountApi {
	return &AccountApi{
		db: db,
	}
}

func (t *AccountApi) StartAPI(g *echo.Group) {
	g.GET("/", t.rootHandler)
	g.GET("/:id", t.queryAccountHandler)
	g.PUT("/", t.createHandler)
}

func (t *AccountApi) rootHandler(ctx echo.Context) error {
	return ctx.Blob(200, "application/json", []byte(`
	GET /account/:id   获取账户信息
	`))
}

func (t *AccountApi) createHandler(ctx echo.Context) error {
	return ctx.Blob(200, "application/json", nil)
}

func (t *AccountApi) queryAccountHandler(ctx echo.Context) error {
	id := ctx.Param("id")
	account, err := t.db.GetAccountByID(id)
	if err != nil {
		return &echo.HTTPError{Code: -1, Internal: err}
	}
	if account == nil {
		return &echo.HTTPError{Code: http.StatusNotFound, Internal: fmt.Errorf("未查询到此交易")}
	}

	return api.DataPackage(0, "success", account, ctx)
}
