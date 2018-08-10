package exchange

import (
	"b1Exchange/pkg/log"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/freebsdly/tools/timer"
)

//
func (p *Exchange) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
	s := fmt.Sprintf(`
	balancePercent  %f
	baseBalance     %f
	quoteBalance    %f
	baseAvaiable    %f
	quoteAvaiable   %f
	askPrice        %f
	bidPrice        %f
	currentTicker   %v
	limitation      %f
	keepRunning     %v
	stat.data       %v
	`, p.balancePercent, p.baseBalance, p.quoteBalance, p.baseAvaiable, p.quoteAvaiable,
		p.askPrice, p.bidPrice, p.currentTicker, p.limitation, p.keepRunning, p.stat.Data)

	resp.Write([]byte(s))
}

// TODO 可以设置一个分发器管道，所有信号都放入这个管道，由分发器负责转发，就像akka
func (p *Exchange) Start() {

	t, err := timer.NewTimer(timer.TIMEWHEEL)
	if err != nil {
		log.Logger.Infof("创建调度器失败, %s", err)
		os.Exit(1)
	}

	log.Logger.Infof("启动调度器")
	t.Start()

	log.Logger.Infof("启动交易服务")
	go p.Exchange()

	log.Logger.Infof("启动资产平衡服务")
	go p.BalanceAccountBalance()

	log.Logger.Infof("启动取消订单服务")
	go p.CancelOrders()

	log.Logger.Infof("启动资产检查服务")
	go p.CheckAccountBalance()

	log.Logger.Infof("启动操作时间统计服务")
	go p.CountTime()

	if p.config.EnableCheckLimitation {
		log.Logger.Infof("启动检查挖矿限量服务")
		go p.CheckOneLimitation()

		// 先检查检查一下限额
		p.checkLimitationChan <- CheckLimitationType
		now := time.Now()
		ts := int64(now.Second() + now.Minute()*60)
		next := 3600 - ts

		log.Logger.Debugf("到下一个小时还有%d秒\n", next)

		t.Add(newRunCheckLimitation(p.checkLimitationChan, CheckLimitationType), uint32(p.config.CheckLimitationInterval/1000), false)
		t.Add(newRunKeepRunning(t, p.checkLimitationChan, KeepRunningType), uint32(next), true)

	}

	log.Logger.Infof("添加定时任务")
	t.Add(newRunExchange(p.checkBalanceChan), uint32(p.config.ExchangeInterval/1000), false)
	t.Add(newRunCancelOrder(p.cancelOrderChan), uint32(p.config.CheckOrderInterval/1000), false)

}

//
func newRunExchange(c chan<- int) *runExchange {
	return &runExchange{
		signChan: c,
	}
}

type runExchange struct {
	signChan chan<- int
}

func (p *runExchange) Run() error {
	p.signChan <- NormalExchangeType
	return nil
}

func (p *runExchange) Stop() error {
	return nil
}

func newRunCancelOrder(c chan<- int) *runCancelOrder {
	return &runCancelOrder{
		signChan: c,
	}
}

type runCancelOrder struct {
	signChan chan<- int
}

func (p *runCancelOrder) Run() error {
	p.signChan <- AllOrderType
	return nil
}

func (p *runCancelOrder) Stop() error {
	return nil
}

type runCheckLimitation struct {
	signChan chan<- int
	sign     int
}

func (p *runCheckLimitation) Run() error {
	p.signChan <- p.sign
	return nil
}

func (p *runCheckLimitation) Stop() error {
	return nil
}

func newRunCheckLimitation(c chan<- int, sign int) *runCheckLimitation {
	return &runCheckLimitation{
		signChan: c,
		sign:     sign,
	}
}

type runKeepRunning struct {
	tr       timer.Timer
	sign     int
	signChan chan<- int
}

func (p *runKeepRunning) Run() error {
	p.signChan <- p.sign
	p.tr.Add(newRunCheckLimitation(p.signChan, p.sign), uint32(3600), false)
	return nil
}

func (p *runKeepRunning) Stop() error {
	return nil
}

func newRunKeepRunning(t timer.Timer, ch chan<- int, si int) *runKeepRunning {
	return &runKeepRunning{
		tr:       t,
		sign:     si,
		signChan: ch,
	}
}
