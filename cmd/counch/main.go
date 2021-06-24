package main

import (
	"log"
	"os"

	"github.com/urfave/cli/v2"
	"github.com/wupeaking/pbft_impl/cmd/account"
	"github.com/wupeaking/pbft_impl/node"
)

func main() {
	app := &cli.App{
		Name:    "counch 贝壳-一个区块链平台",
		Version: "v0.0.1",
		Usage:   "counch --help 显示更多使用说明",
		Commands: []*cli.Command{
			{
				Name:        "account",
				Aliases:     []string{"account"},
				Usage:       "贝壳账户系统",
				Description: "创建/解析/校验账户信息",
				Subcommands: []*cli.Command{
					{
						Name:    "create",
						Aliases: []string{"c"},
						Usage:   "创建一个新的账户",
						Flags: []cli.Flag{
							&cli.StringFlag{Name: "password", Usage: "密码", DefaultText: "123456"},
						},
						Action: func(c *cli.Context) error {
							return account.GenerateAccount(c.String("password"))
						},
					},
					{
						Name:    "pub2address",
						Aliases: []string{"p2a"},
						Usage:   "公钥转换地址",
						Flags: []cli.Flag{
							&cli.StringFlag{Name: "pub", Usage: "公钥", Required: true},
						},
						Action: func(c *cli.Context) error {
							return account.PublicKeyToAddress(c.String("pub"))
						},
					},
					{
						Name:  "balance",
						Usage: "查询账户余额",
						Flags: []cli.Flag{
							&cli.StringFlag{Name: "api", Usage: "api地址", DefaultText: "http://localhost:8088"},
						},
						Action: func(c *cli.Context) error {
							return account.Balance(c.String("api"))
						},
					},
					{
						Name:  "transfer",
						Usage: "转账",
						Flags: []cli.Flag{
							&cli.StringFlag{Name: "api", Usage: "api地址", DefaultText: "http://localhost:8088"},
							&cli.StringFlag{Name: "password", Usage: "账户密码", DefaultText: "123456"},
							&cli.StringFlag{Name: "to", Usage: "接收方地址", Required: true},
							&cli.IntFlag{Name: "index", Usage: "账户序号", DefaultText: "-1"},
							&cli.StringFlag{Name: "address", Usage: "账户地址"},
							&cli.Int64Flag{Name: "amount", Usage: "转账金额"},
						},
						Action: func(c *cli.Context) error {
							return account.Transfer(c.String("api"), c.String("to"),
								c.String("password"), c.String("address"), c.Int("index"), c.Int64("amount"))
						},
					},
					{
						Name:  "list",
						Usage: "列出所有账户",
						Flags: []cli.Flag{
							&cli.StringFlag{Name: "password", Usage: "账户密码"},
						},
						Action: func(c *cli.Context) error {
							return account.List(c.String("password"))
						},
					},
				},
			},
		},
		Action: func(c *cli.Context) error {
			node.New().Run()
			return nil
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
