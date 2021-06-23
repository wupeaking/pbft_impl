package api

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo"
	"github.com/wupeaking/pbft_impl/common/config"
)

// API服务模块
type API struct {
	*echo.Echo
	port string
}

func New(cfg *config.Configure) *API {
	api := &API{}
	api.Echo = echo.New()
	api.Echo.HideBanner = true
	api.Echo.HTTPErrorHandler = httpErrorHandler
	// WebApp.Use(func(h echo.HandlerFunc) echo.HandlerFunc {
	// 	return func(c echo.Context) error {
	// 		cc := &CustomContext{c}
	// 		return h(cc)
	// 	}
	// })
	if cfg.WebCfg.Port == 0 {
		api.port = "8088"
	} else {
		api.port = fmt.Sprintf("%d", cfg.WebCfg.Port)
	}
	return api
}

func (api *API) Start() {
	api.Echo.Start(":" + api.port)
}

func (api *API) DefaultHandler(ctx echo.Context) error {
	ctx.Blob(200, "application/json", []byte(`
	/blockchain/   区块链服务
	/consensus/  共识服务
	/ws/   全局状态
	
	`))
	return nil
}

func httpErrorHandler(err error, c echo.Context) {
	var (
		code = http.StatusInternalServerError
		msg  interface{}
	)
	if he, ok := err.(*echo.HTTPError); ok {
		code = he.Code
		msg = he.Message
		if he.Internal != nil {
			msg = he.Internal.Error()
		}
	} else {
		msg = err.Error()
	}

	// Send response
	if !c.Response().Committed {
		if c.Request().Method == echo.HEAD {
			err = c.NoContent(code)
		} else {
			err = c.JSON(200, map[string]interface{}{"code": code, "data": "", "message": msg})
		}
	}
}

func DataPackage(code int, message string, data interface{}, ctx echo.Context) error {
	resp := struct {
		Code    int         `json:"code"`
		Message string      `json:"message"`
		Data    interface{} `json:"data"`
	}{
		Code:    code,
		Message: message,
		Data:    data,
	}
	return ctx.JSON(200, resp)
}
