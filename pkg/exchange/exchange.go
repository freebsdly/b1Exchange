package exchange

import (
	"b1Exchange/pkg/api"
	"b1Exchange/pkg/log"
	"b1Exchange/pkg/model"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	NormalExchangeType  int = iota // 正常交易模式
	BalanceExchangeType            // 平衡资产交易模式
	BidExchangeType
	AskExchangeType
)

const (
	AskOrderTypeName = "ASK"
	BidOrderTypeName = "BID"
)

const (
	BidOrderType int = iota // bid订单，即买单
	AskOrderType            // ask订单，即卖单
	AllOrderType
)

const (
	CheckLimitationType int = iota
	KeepRunningType
)

// 交易客户端
type Exchange struct {
	symbolPair      *model.SymbolPair
	priceFormat     string
	amountFormat    string
	balancePercent  float64
	baseBalance     float64
	quoteBalance    float64
	baseAvaiable    float64
	quoteAvaiable   float64
	askPrice        float64
	bidPrice        float64
	currentBalances map[string]*model.Balance
	currentTicker   *model.MarketTickerResponeBody
	b1client        *api.Client

	config *model.Configuration
	sync.RWMutex

	limitation  float64
	keepRunning bool
	stat        *model.OneHourlyLimitationResponeBody

	checkBalanceChan    chan int // 检查账户余额信号管道
	balanceChan         chan int // 平衡资产信号管道
	exchangeChan        chan int // 交易信号管道
	cancelOrderChan     chan int // 取消订单信号管道
	checkLimitationChan chan int // 检查挖矿限量管道

	// 耗时统计管道
	checkBalanceTimeChan chan int64
	balanceTimeChan      chan int64
	exchangeTimeChan     chan int64
	cancelOrderTimeChan  chan int64

	exchangeLockChan    chan bool
	cancelOrderLockChan chan bool
}

// 创建新的交易客户端
func NewExchange(cfg *model.Configuration) (*Exchange, error) {
	client := api.NewClient(cfg.EndPoint, cfg.AppKey, cfg.AppSecret, cfg.RequestTimeout)
	markets, err := client.GetAllMarkets()
	if err != nil {
		return nil, err
	}

	var (
		mmap  = make(map[string]*model.SymbolPair)
		exist bool
		pair  = strings.ToUpper(cfg.SymbolPair)
	)
	for _, p := range markets.Data {
		mmap[p.Name] = p
	}

	_, exist = mmap[pair]
	if !exist {
		return nil, fmt.Errorf("交易对 %s 不存在", pair)
	}

	log.Logger.Infof("基础资产: %s, 精度: %d, 交易资产: %s, 精度: %d", mmap[pair].BaseAsset.Name, mmap[pair].BaseScale,
		mmap[pair].QuoteAsset.Name, mmap[pair].QuoteScale)

	lmt, err := client.OneLimitation()
	if err != nil {
		log.Logger.Infof("获取限额失败\n")
		return nil, err
	}

	limitation := lmt.Data / 24.0
	log.Logger.Infof("当前每小时限额：%f\n", limitation)

	return &Exchange{
		symbolPair:           mmap[pair],
		priceFormat:          fmt.Sprintf("%%.%df", mmap[pair].BaseScale),
		amountFormat:         fmt.Sprintf("%%.%df", mmap[pair].QuoteScale),
		balancePercent:       float64(cfg.BalancePercent) / 100.0,
		currentBalances:      make(map[string]*model.Balance),
		currentTicker:        new(model.MarketTickerResponeBody),
		b1client:             client,
		config:               cfg,
		keepRunning:          true,
		limitation:           limitation,
		stat:                 new(model.OneHourlyLimitationResponeBody),
		checkBalanceChan:     make(chan int, 1),
		balanceChan:          make(chan int, 1),
		exchangeChan:         make(chan int, 1),
		cancelOrderChan:      make(chan int, 1),
		checkLimitationChan:  make(chan int, 1),
		checkBalanceTimeChan: make(chan int64, 1),
		balanceTimeChan:      make(chan int64, 1),
		exchangeTimeChan:     make(chan int64, 1),
		cancelOrderTimeChan:  make(chan int64, 1),

		exchangeLockChan:    make(chan bool, 1),
		cancelOrderLockChan: make(chan bool, 1),
	}, nil
}

//
func (p *Exchange) Ask(nonce int64, market, price, amount string) (*model.Order, error) {
	var parms = map[string]string{
		"market_id": market,
		"price":     price,
		"amount":    amount,
		"side":      "ASK",
	}

	return p.b1client.CreateOrder(nonce, parms)
}

//
func (p *Exchange) Bid(nonce int64, market, price, amount string) (*model.Order, error) {
	var parms = map[string]string{
		"market_id": market,
		"price":     price,
		"amount":    amount,
		"side":      "BID",
	}

	return p.b1client.CreateOrder(nonce, parms)
}

//
func (p *Exchange) CheckOneLimitation() {
	var (
		stat        *model.OneHourlyLimitationResponeBody
		err         error
		sign        int
		keepRunning bool
	)

	for {
		select {
		case sign = <-p.checkLimitationChan:
			stat, err = p.b1client.OneHourlyStatistic()
			if err != nil {
				log.Logger.Infof("检查系统当前小时挖矿量失败，%s,将使用上一次获取结果进行检查\n", err)
				stat = p.stat
			}

			if sign == CheckLimitationType {
				log.Logger.Infof("当前每小时挖矿奖励: %f, 当前每小时邀请奖励: %f \n", stat.Data.TradeMineOne, stat.Data.InviteMineOne)
				pct := (stat.Data.TradeMineOne + stat.Data.InviteMineOne) * 100.0 / p.limitation
				log.Logger.Infof("当前小时已挖矿量占限额比例: %2.2f%%", pct)
				if pct >= float64(p.config.OneHourlyLimitationPercent) {
					log.Logger.Infof("当前小时已挖矿量已超限额的%f，设置停止挖矿", p.config.OneHourlyLimitationPercent)
					keepRunning = false
				} else {
					keepRunning = true
				}
			} else {
				log.Logger.Debugf("收到keepRunning信号")
				p.keepRunning = true
			}

			log.Logger.Debugf("将要设置p.keepRunging为%v\n", keepRunning)
			p.Lock()
			p.keepRunning = keepRunning
			p.Unlock()

			p.stat = stat

		}
	}
}

// 检查账户资产
func (p *Exchange) CheckAccountBalance() {
	var keepRunning bool

	for {
		// 获取账户
		select {
		case <-p.checkBalanceChan:
			p.RLock()
			keepRunning = p.keepRunning
			p.RUnlock()

			if !keepRunning {
				log.Logger.Debugf("已达到限额，停止挖矿")
				break
			}

			go func() {
				var (
					err                error
					start              int64
					end                int64
					dtime              int64
					baseLockedBalance  float64
					quoteLockedBalance float64

					base    = new(model.Balance)
					quote   = new(model.Balance)
					account = new(model.AccountResponeBody)
					bflag   int
					qflag   int
					code    int
					tk      *time.Ticker
				)

				log.Logger.Infof("开始检查账户资产")
				start = time.Now().UnixNano()
				defer func() {
					end = time.Now().UnixNano()
					p.checkBalanceTimeChan <- end - start
				}()

				account, err = p.b1client.GetAccounts(start)
				if err != nil {
					log.Logger.Errorf("获取账户资产失败. %s", err)
					return
				}

				for _, v := range account.Data {
					p.currentBalances[v.AssetUUID] = v
				}

				base = p.currentBalances[p.symbolPair.BaseAsset.UUID]
				quote = p.currentBalances[p.symbolPair.QuoteAsset.UUID]

				p.baseBalance, err = strconv.ParseFloat(base.Balance, 10)
				if err != nil {
					log.Logger.Errorf("转换%s资产数量为float类型失败. %s", p.symbolPair.BaseAsset.Name, err)
					return
				}

				baseLockedBalance, err = strconv.ParseFloat(base.LockedBalance, 10)
				if err != nil {
					log.Logger.Errorf("转换%资产已锁定数量为float类型失败. %s", p.symbolPair.BaseAsset.Name, err)
					return
				}

				p.quoteBalance, err = strconv.ParseFloat(quote.Balance, 10)
				if err != nil {
					log.Logger.Errorf("转换%s资产数量为float类型失败. %s", p.symbolPair.QuoteAsset.Name, err)
					return
				}

				quoteLockedBalance, err = strconv.ParseFloat(quote.LockedBalance, 10)
				if err != nil {
					log.Logger.Errorf("转换%资产已锁定数量为float类型失败. %s", p.symbolPair.QuoteAsset.Name, err)
					return
				}

				p.baseAvaiable = p.baseBalance - baseLockedBalance
				p.quoteAvaiable = p.quoteBalance - quoteLockedBalance

				log.Logger.Debugf("当前 %s 资产: %f, 可用: %f",
					p.symbolPair.BaseAsset.Name, p.baseBalance, p.baseAvaiable)
				log.Logger.Debugf("当前 %s 资产: %f, 可用: %f",
					p.symbolPair.QuoteAsset.Name, p.quoteBalance, p.quoteAvaiable)

				p.currentTicker, err = p.b1client.GetTicker(p.symbolPair.Name)
				if err != nil {
					log.Logger.Errorf("获取行情数据失败. %s", err)
					return
				}

				// 判断可用账户余额
				if p.baseAvaiable >= p.config.ExchangeAmount {
					bflag = 20
				} else {
					bflag = 10
				}

				p.askPrice, err = strconv.ParseFloat(p.currentTicker.Data.Ask.Price, 10)
				if err != nil {
					log.Logger.Errorf("转换当前ask价格为float类型失败")
					return
				}

				p.bidPrice, err = strconv.ParseFloat(p.currentTicker.Data.Bid.Price, 10)
				if err != nil {
					log.Logger.Errorf("转换当前bid价格为float类型失败")
					return
				}

				log.Logger.Debugf("current ask price %f", p.askPrice)
				if p.quoteAvaiable >= (p.askPrice * p.config.ExchangeAmount) {
					qflag = 2
				} else {
					qflag = 1
				}

				end = time.Now().UnixNano()
				dtime = (end - start) / 1000000
				if dtime < p.config.CheckBalanceRelayTime {
					tk = time.NewTicker(time.Duration(p.config.CheckBalanceRelayTime-dtime) * time.Millisecond)
					log.Logger.Infof("检查订单延时 %d 毫秒", p.config.CheckBalanceRelayTime-dtime)
					<-tk.C
					tk.Stop()
				}

				code = bflag + qflag
				switch code {
				case 22:
					log.Logger.Infof("账户可用资产足够，准备进行买卖")
					p.exchangeChan <- NormalExchangeType
					break
				case 12:
					log.Logger.Infof("账户%s可用资产不足，准备平衡该资产", p.symbolPair.BaseAsset.Name)
					if p.config.BalanceAccountBalance {
						p.balanceChan <- 0
					}
					break
				case 21:
					log.Logger.Infof("账户%s可用资产不足，准备平衡该资产", p.symbolPair.QuoteAsset.Name)
					if p.config.BalanceAccountBalance {
						p.balanceChan <- 0
					}
					break
				case 11:
					log.Logger.Infof("账户可用资产不足，准备平衡该资产")
					if p.config.BalanceAccountBalance {
						p.balanceChan <- 0
					}
					break
				}
			}()
		}
	}
}

// 统计各个操作的时间
func (p *Exchange) CountTime() {
	var (
		checkBalanceTime int64
		exchangeTime     int64
		cancelorderTime  int64
		balanceTime      int64
	)
	for {
		select {
		case checkBalanceTime = <-p.checkBalanceTimeChan:
			log.Logger.Infof("检查账户资产使用时间: %d 毫秒", checkBalanceTime/1000000)
		case exchangeTime = <-p.exchangeTimeChan:
			log.Logger.Infof("交易使用时间: %d 毫秒", exchangeTime/1000000)
		case cancelorderTime = <-p.cancelOrderTimeChan:
			log.Logger.Infof("检查订单使用时间: %d 毫秒", cancelorderTime/1000000)
		case balanceTime = <-p.balanceTimeChan:
			log.Logger.Infof("平衡账户使用时间: %d 毫秒", balanceTime/1000000)
		}
	}
}

// 刷单时在取一次行情
func (p *Exchange) Exchange() {
	var (
		lock  bool = false
		ecode int
	)

	for {
		select {
		case lock = <-p.exchangeLockChan:
			if lock {
				log.Logger.Infof("锁定自动交易")
			} else {
				log.Logger.Infof("解锁自动交易")
			}
		case ecode = <-p.exchangeChan:
			if lock {
				log.Logger.Infof("自动交易已锁定")
				break
			}

			go func(code int) {
				var (
					err           error
					start         int64
					end           int64
					nonce         int64
					price         string
					amount        string
					askPrice      float64
					currentTicker *model.MarketTickerResponeBody
					a             float64
				)
				log.Logger.Infof("开始进行交易")
				start = time.Now().UnixNano()
				defer func() {
					end = time.Now().UnixNano()
					p.exchangeTimeChan <- end - start
				}()

				if code == NormalExchangeType {
					a = float64(1)
				} else {
					a = float64(p.config.BalanceExchangePercent) / 100.0
				}

				currentTicker, err = p.b1client.GetTicker(p.symbolPair.Name)
				if err != nil {
					log.Logger.Errorf("获取行情数据失败. %s", err)
					return
				}
				askPrice, err = strconv.ParseFloat(currentTicker.Data.Ask.Price, 10)
				if err != nil {
					log.Logger.Errorf("转换当前ask价格为float类型失败")
					return
				}

				price = fmt.Sprintf(p.priceFormat, math.Abs(askPrice-p.config.ExpectDiffrentValue))
				amount = fmt.Sprintf(p.amountFormat, p.config.ExchangeAmount*a)
				nonce = time.Now().UnixNano()
				go func() {
					_, err := p.Bid(nonce, p.symbolPair.UUID, price, amount)
					log.Logger.Infof("交易时创建BID买入订单price: %s, amount: %s", price, amount)
					if err != nil {
						log.Logger.Errorf("创建BID买入订单失败. %s", err)
					}
				}()
				go func() {
					_, err := p.Ask(nonce+1, p.symbolPair.UUID, price, amount)
					log.Logger.Infof("交易时时创建ASK卖出订单price: %s, amount: %s", price, amount)
					if err != nil {
						log.Logger.Errorf("创建ASK卖出订单失败. %s", err)
					}
				}()
			}(ecode)
		}
	}

}

// 取消订单
func (p *Exchange) CancelOrders() {

	var (
		lock      bool = false
		orderType int
	)
	for {
		select {
		case lock = <-p.cancelOrderLockChan:
			if lock {
				log.Logger.Infof("锁定自动撤单")
			} else {
				log.Logger.Infof("解锁自动撤单")
			}
		case orderType = <-p.cancelOrderChan:
			if lock {
				log.Logger.Infof("自动撤单已被锁定")
				break
			}

			go func(otype int) {
				var (
					ot          int
					err         error
					start       int64
					end         int64
					dTime       int64
					serverTime  int64
					nonce       int64
					cancelDtime = float64(p.config.CancelOrderDiffrentTime)
					orders      *model.OrderListResponeBody

					parms = map[string]string{
						"market_id": p.symbolPair.UUID,
						"first":     fmt.Sprintf("%d", p.config.CheckOrderNumber),
					}
					states []string = p.config.CancelOrderTypes

					tk = time.NewTicker(time.Duration(p.config.CancelOrderInterval) * time.Millisecond)
				)
				// 锁定交易
				if p.config.CancelOrderLockExchange {
					p.exchangeLockChan <- true
					defer func() {
						p.exchangeLockChan <- false
					}()
				}

				//
				start = time.Now().UnixNano()
				defer func() {
					end = time.Now().UnixNano()
					p.cancelOrderTimeChan <- end - start
				}()

				for _, state := range states {
					log.Logger.Infof("开始检查 %s 状态订单", state)
					nonce = time.Now().UnixNano()
					parms["state"] = strings.ToUpper(state)
					orders, err = p.b1client.GetOrders(nonce+1, parms)
					if err != nil {
						log.Logger.Errorf("获取订单列表失败. %s\n", err)
						continue
					}

					serverTime, err = p.b1client.Ping()
					if err != nil {
						log.Logger.Errorf("获取交易所服务器时间失败. %s", err)
						continue
					}

					for _, order := range orders.Data.Edges {
						switch strings.ToUpper(order.Node.Side) {
						case BidOrderTypeName:
							ot = BidOrderType
						case AskOrderTypeName:
							ot = AskOrderType
						}

						switch otype {
						case BidOrderType:
							if ot != BidOrderType {
								continue
							}
							break
						case AskOrderType:
							if ot != AskOrderType {
								continue
							}
							break
						default:
							break
						}

						dTime = (serverTime - order.Node.InsertedAt.UnixNano()) / 1000000
						if math.Abs(float64(dTime)) > cancelDtime {
							// cancel order
							nonce = time.Now().UnixNano()
							log.Logger.Infof("服务器当前时间大于订单 %s 创建时间%d毫秒，订单超时，开始取消", order.Node.Id, dTime)
							_, err = p.b1client.CancelOrder(nonce, order.Node.Id)
							if err != nil {
								log.Logger.Infof("取消订单 %s 失败. %s", order.Node.Id, err)
							}
							<-tk.C
						}
					}
				}
			}(orderType)
		}
	}
}

// 平衡账户资产
func (p *Exchange) BalanceAccountBalance() {

	for {
		select {
		case <-p.balanceChan:
			log.Logger.Infof("开始平衡资产")
			go func() {
				var (
					err           error
					start         int64
					end           int64
					nonce         int64
					number        float64
					bflag         int
					qflag         int
					code          int
					askPrice      float64
					bidPrice      float64
					price         string
					amount        string
					currentTicker *model.MarketTickerResponeBody
				)
				// 锁定自动撤单
				if p.config.BalanceLockCancelOrder {
					p.cancelOrderLockChan <- true
					defer func() {
						p.cancelOrderLockChan <- false
					}()
				}

				//
				start = time.Now().UnixNano()
				defer func() {
					end = time.Now().UnixNano()
					p.balanceTimeChan <- end - start
				}()

				// 这里使用获取账户信息时获取到的行情
				if p.baseBalance > p.config.ExchangeAmount {
					bflag = 20
				} else {
					bflag = 10
				}

				if p.quoteBalance > p.askPrice*p.config.ExchangeAmount {
					qflag = 2
				} else {
					qflag = 1
				}

				code = bflag + qflag

				// 平衡时在获取一次行情
				switch code {
				case 12:
					// 补充base currency
					log.Logger.Infof("账户 %s 总资产不足，准备平衡该资产", p.symbolPair.BaseAsset.Name)
					number = p.askPrice * p.config.ExchangeAmount * p.balancePercent

					if p.quoteAvaiable < number {
						log.Logger.Infof("账户 %s 可用资产不足以平衡资产，尝试取消订单", p.symbolPair.QuoteAsset.Name)
						p.cancelOrderChan <- AskOrderType
						break
					}

					currentTicker, err = p.b1client.GetTicker(p.symbolPair.Name)
					if err != nil {
						log.Logger.Errorf("获取行情数据失败. %s", err)
						break
					}

					askPrice, err = strconv.ParseFloat(currentTicker.Data.Ask.Price, 10)
					if err != nil {
						log.Logger.Errorf("转换当前ask价格为float类型失败")
						break
					}

					price = fmt.Sprintf(p.priceFormat, askPrice)
					amount = fmt.Sprintf(p.amountFormat, p.config.ExchangeAmount*p.balancePercent)
					nonce = time.Now().UnixNano()
					_, err = p.Bid(nonce, p.symbolPair.UUID, price, amount)
					log.Logger.Infof("平衡资产时创建BID买入订单price: %s, amount: %s", price, amount)
					if err != nil {
						log.Logger.Errorf("平衡资产时创建BID买入订单失败. %s", err)
					}
				case 22:
					log.Logger.Infof("账户总资产足够，取消订单来平衡账户")
					if p.config.BalanceLockCancelOrder {
						p.cancelOrderLockChan <- false
					}
					p.cancelOrderChan <- AllOrderType
					// 取消订单会多次调用接口，这里在进行刷单可能会导致接口调用超出限制
					p.exchangeChan <- BalanceExchangeType
					break
				case 21:
					// 补充quote currency
					log.Logger.Infof("账户 %s 总资产不足，准备平衡该资产", p.symbolPair.QuoteAsset.Name)
					number = p.config.ExchangeAmount * p.balancePercent
					if p.baseAvaiable < number {
						log.Logger.Infof("账户 %s 可用资产不足以平衡资产,尝试取消订单", p.symbolPair.BaseAsset.Name)
						p.cancelOrderChan <- BidOrderType
						break
					}

					currentTicker, err = p.b1client.GetTicker(p.symbolPair.Name)
					if err != nil {
						log.Logger.Errorf("获取行情数据失败. %s", err)
						break
					}

					bidPrice, err = strconv.ParseFloat(currentTicker.Data.Bid.Price, 10)
					if err != nil {
						log.Logger.Errorf("转换当前bid价格为float类型失败")
						break
					}
					price = fmt.Sprintf(p.priceFormat, bidPrice)
					amount = fmt.Sprintf(p.amountFormat, p.config.ExchangeAmount*p.balancePercent)
					nonce = time.Now().UnixNano()
					_, err = p.Ask(nonce, p.symbolPair.UUID, price, amount)
					log.Logger.Infof("平衡资产时创建ASK卖出订单price: %s, amount: %s", price, amount)
					if err != nil {
						log.Logger.Errorf("平衡资产时创建ASK卖出订单失败. %s", err)
					}

					break
				case 11:
					// 减小sell number
					log.Logger.Infof("账户总资产不足，降低买卖数量%f 到 %f",
						p.config.ExchangeAmount, p.config.ExchangeAmount*float64(p.config.BalanceExchangePercent)/100.0)
					p.config.ExchangeAmount = p.config.ExchangeAmount * float64(p.config.BalanceExchangePercent) / 100.0
					break
				}
			}()
		}

	}
}
