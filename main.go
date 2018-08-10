// b1Exchange project main.go
package main

import (
	"b1Exchange/pkg/conf"
	"b1Exchange/pkg/exchange"
	"b1Exchange/pkg/log"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"
)

func main() {
	var cfgPath string
	flag.StringVar(&cfgPath, "config", "conf/b1.yaml", "configuration file")
	flag.Parse()

	cfg, err := conf.Parse(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}

	log.Init(cfg.LogFile, cfg.LogLevel)

	var (
		ex *exchange.Exchange
	)

	for {
		ex, err = exchange.NewExchange(cfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "创建交易客户端失败, %s\n", err)
			fmt.Fprintf(os.Stderr, "等待%d毫秒再次尝试\n", cfg.CreateExchangeClientWaitTime)
			time.Sleep(time.Duration(cfg.CreateExchangeClientWaitTime) * time.Millisecond)

		} else {
			fmt.Fprintf(os.Stderr, "创建交易客户端成功, 启动交易客户端\n")
			break
		}
	}

	ex.Start()

	http.Handle("/info", ex)

	if err = http.ListenAndServe("0.0.0.0:18080", nil); err != nil {
		log.Logger.Errorf("%s\n", err)
	}
}
