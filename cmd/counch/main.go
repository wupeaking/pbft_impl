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
							&cli.BoolFlag{Name: "v", Usage: "是否在屏幕打印", Value: true},
							&cli.StringFlag{Name: "s", Usage: "保存文件名称"},
						},
						Action: func(c *cli.Context) error {
							return account.GenerateAccount(c.String("s"), c.Bool("v"))
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
