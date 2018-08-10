package model

import (
	"fmt"
	"strings"
	"time"
)

const (
	OrderPendingState  = "PENDING"
	OrderFilledState   = "FILLED"
	OrderCanceledState = "CANCLED"
)

type Configuration struct {
	EndPoint                     string   `yaml:"endpoint"`
	AppKey                       string   `yaml:"appkey"`
	AppSecret                    string   `yaml:"appsecret"`
	SymbolPair                   string   `yaml:"symbol_pair"`
	OneHourlyLimitationPercent   int      `yaml:"one_hourly_limitation_percent"`
	CheckLimitationInterval      int64    `yaml:"check_limitation_interval"`
	EnableCheckLimitation        bool     `yaml:"enable_check_limitation"`
	ExchangeAmount               float64  `yaml:"exchange_amount"`
	ExchangeInterval             int64    `yaml:"exchange_interval"`
	RequestTimeout               int64    `yaml:"request_timeout"`
	BalanceAccountBalance        bool     `yaml:"balance_account_balance"`
	BalancePercent               int      `yaml:"balance_percent"`
	BalanceExchange              bool     `yaml:"balance_exchange"`
	BalanceExchangePercent       int      `yaml:"balance_exchange_percent"`
	ExpectDiffrentValue          float64  `yaml:"expect_diffrent_value"`
	CheckOrderInterval           int64    `yaml:"check_order_interval"`
	CheckOrderNumber             int      `yaml:"check_order_number"`
	CancelOrderDiffrentTime      int64    `yaml:"cancel_order_diffrent_time"`
	CancelOrderTypes             []string `yaml:"check_order_type"`
	CancelOrderInterval          int64    `yaml:"cancel_order_interval"`
	CancelOrderLockExchange      bool     `yaml:"cancel_order_lock_exchange"`
	CheckBalanceRelayTime        int64    `yaml:"check_balance_relay_time"`
	CreateExchangeClientWaitTime int64    `yaml:"create_exchange_client_wait_time"`
	BalanceLockCancelOrder       bool     `yaml:"balance_lock_cancel_order"`
	LogFile                      string   `yaml:"log_file"`
	LogLevel                     string   `yaml:"log_level"`
}

func (p *Configuration) Check() error {
	if p.EndPoint == "" {
		return fmt.Errorf("endpoint必须设置")
	}

	if p.AppKey == "" {
		return fmt.Errorf("appkey必须设置")
	}

	if p.AppSecret == "" {
		return fmt.Errorf("appsecret必须设置")
	}

	if p.SymbolPair == "" {
		return fmt.Errorf("symbol 交易対必须设置")
	}

	if p.OneHourlyLimitationPercent <= 0 || p.OneHourlyLimitationPercent > 100 {
		return fmt.Errorf("每小时挖矿限量百分比必须大于0,同时小于等于100")
	}

	if p.CheckLimitationInterval == 0 {
		return fmt.Errorf("检查每小时挖矿限量时间间隔不能为0")
	}

	if p.ExchangeAmount == 0 {
		return fmt.Errorf("sellnumber 不能为0或者未设置")
	}

	if p.BalancePercent <= 0 || p.BalancePercent > 100 {
		return fmt.Errorf("balance_percent 资产平衡时交易数量占exchange_amount百分比必须大于0小于等于100")
	}

	if p.BalanceExchangePercent <= 0 || p.BalanceExchangePercent > 100 {
		return fmt.Errorf("balance_percent 资产不足时交易数量占exchange_amount百分比必须大于0小于等于100")
	}

	//	if p.ExpectDiffrentValue == 0 {
	//		return fmt.Errorf("expect_diffrent_value 期望买卖差价不能为0或则未设置")
	//	}

	if p.CancelOrderDiffrentTime == 0 {
		return fmt.Errorf("cancel_order_diffrent_time 订单创建时间与交易所服务器时间差不能等于0或则未设置")
	}

	if p.ExchangeInterval == 0 {
		return fmt.Errorf("exchange_interval 自动交易频率不能为0或者未设置")
	}

	if p.CheckOrderNumber == 0 {
		return fmt.Errorf("check_order_number 订单列表数量不能为0或者未设置")
	}

	if p.CancelOrderTypes == nil {
		return fmt.Errorf("cancel_order_type 必须设置")
	}

	if len(p.CancelOrderTypes) == 0 {
		return fmt.Errorf("order_type 不能为空")
	}

	for _, v := range p.CancelOrderTypes {
		switch strings.ToUpper(v) {
		case OrderCanceledState, OrderFilledState, OrderPendingState:
			break
		default:
			return fmt.Errorf("cancel_order_type must be %s/%s/%s",
				OrderCanceledState, OrderPendingState, OrderFilledState)
			break
		}
	}

	if p.CreateExchangeClientWaitTime == 0 {
		p.CreateExchangeClientWaitTime = 2000
	}

	if p.LogFile == "" {
		p.LogFile = "log/b1.log"
	}

	if p.LogLevel == "" {
		p.LogLevel = "error"
	}

	return nil
}

type AccountResponeBody struct {
	Data   []*Balance     `json:"data"`
	Errors []ErrorMessage `json:"errors"`
}

type Balance struct {
	AssetUUID     string `json:"asset_uuid"`
	Balance       string `json:"balance"`
	LockedBalance string `json:"locked_balance"`
}

//
type AllTickersResponeBody struct {
	Data   []*Ticker      `json:"data"`
	Errors []ErrorMessage `json:"errors"`
}

//
type MarketTickerResponeBody struct {
	Data   *Ticker        `json:"data"`
	Errors []ErrorMessage `json:"errors"`
}

// 行情结构体
type Ticker struct {
	MarketUUID      string       `json:"market_uuid"`
	Bid             *PriceAmount `json:"bid"`
	Ask             *PriceAmount `json:"ask"`
	Open            string       `json:"open"`
	Close           string       `json:"close"`
	High            string       `json:"high"`
	Low             string       `json:"low"`
	Volume          string       `json:"volume"`
	DailyChange     string       `json:"daily_change"`
	DailyChangePerc string       `json:"daily_change_perc"`
}

type PriceAmount struct {
	Price  string `json:"price"`
	Amount string `json:"amount"`
}

type Trade struct {
	TradeId    string `json:"trade_id"`
	MarketUUID string `json:"market_uuid"`
	Price      string `json:"price"`
	Amount     string `json:"amount"`
	TakerSide  string `json:"taker_side"`
}

type Withdrawal struct {
	Id            string `json:"id"`
	CustomerId    string `json:"customer_id"`
	AssetUUID     string `json:"asset_uuid"`
	Amount        string `json:"amont"`
	State         string `json:"state"`
	RecipientId   string `json:"recipient_id"`
	CompletedAt   string `json:"completed_at"`
	InsertedAt    string `json:"inserted_at"`
	IsInternal    string `json:"is_internal"`
	TargetAddress string `json:"target_address"`
	Note          string `json:"note"`
}

type Deposit struct {
	Id          string `json:"id"`
	CustomerId  string `json:"customer_id"`
	AssetUUID   string `json:"asset_uuid"`
	Amount      string `json:"amont"`
	State       string `json:"state"`
	Note        string `json:"note"`
	TxId        string `json:"txid"`
	ConfirmedAt string `json:"comfiremed_at"`
	InsertedAt  string `json:"inserted_at"`
	confirms    int    `json:"confirms"`
}

type Page struct {
	StartCursor     string `json:"start_cursor"`
	EndCursor       string `json:"end_cursor"`
	HasNextPage     bool   `json:"has_next_page"`
	HasPreviousPage bool   `json:"has_previous_page"`
}

type PingResponeBody struct {
	Timestamp int64 `json:"timestamp"`
}

type MarketResponeBody struct {
	Data   []*SymbolPair  `json:"data"`
	Errors []ErrorMessage `json:"errors"`
}

//  {
//    "uuid": "d2185614-50c3-4588-b146-b8afe7534da6",
//    "quoteScale": 8,
//    "quoteAsset": {
//      "uuid": "0df9c3c3-255a-46d7-ab82-dedae169fba9",
//      "symbol": "BTC",
//      "name": "Bitcoin"
//    },
//    "name": "BTG-BTC",
//    "baseScale": 4,
//    "baseAsset": {
//      "uuid": "5df3b155-80f5-4f5a-87f6-a92950f0d0ff",
//      "symbol": "BTG",
//      "name": "Bitcoin Gold"
//    }
//  }
// scale 即 价格或amount的数值精度
type SymbolPair struct {
	UUID       string `json:"uuid"`
	QuoteScale int    `json:"quoteScale"`
	QuoteAsset *Asset `json:"quoteAsset"`
	Name       string `json:"name"`
	BaseScale  int    `json:"baseScale"`
	BaseAsset  *Asset `json:"baseAsset"`
}

type Asset struct {
	UUID   string `json:"uuid"`
	Symbol string `json:"symbol"`
	Name   string `json:"name"`
}

//id	String	id of order
//market_uuid	String	uuid of market
//price	String	order price
//amount	String	order amount
//filled_amount	String	already filled amount
//avg_deal_price	String	average price of the deal
//side	String	order side, one of ASK/BID
//state	String	order status, one of "FILLED"/"PENDING"/"CANCLED"
type Order struct {
	Id           string    `json:"id"`
	MarketId     string    `json:"market_id"`
	MarketUUID   string    `json:"market_uuid"`
	Price        string    `json:"price"`
	Amount       string    `json:"amount"`
	FilledAmount string    `json:"filled_amount"`
	AvgDealPrice string    `json:"avg_deal_price"`
	Side         string    `json:"side"`
	State        string    `json:"state"`
	UpdatedAt    time.Time `json:"updated_at"`
	InsertedAt   time.Time `json:"inserted_at"`
}

//{
//  "edges": [
//    {
//      "node": {
//        "id": 10,
//        "market_id": "ETH-BTC",
//        "price": "10.00",
//        "amount": "10.00",
//        "filled_amount": "9.0",
//        "avg_deal_price": "12.0",
//        "side": "ASK",
//        "state": "FILLED"
//      },
//      "cursor": "dGVzdGN1cmVzZQo="
//    }
//  ],
//  "page_info": {
//    "end_cursor": "dGVzdGN1cmVzZQo=",
//    "start_cursor": "dGVzdGN1cmVzZQo=",
//    "has_next_page": true,
//    "has_previous_page": false
//  }
//}
type OrderList struct {
	Edges    []*Edge `json:"edges"`
	PageInfo *Page   `json:"page_info"`
}

type Edge struct {
	Node   *Order `json:"node"`
	Cursor string `json:"cursor"`
}

type OrderListResponeBody struct {
	Data   *OrderList      `json:"data"`
	Errors []*ErrorMessage `json:"errors"`
}

type OrderResponeBody struct {
	Data   *Order          `json:"data"`
	Errors []*ErrorMessage `json:"errors`
}

type ErrorMessage struct {
	Message string `json:"message"`
	Code    int    `json:"code"`
}

//{
//  "data": {
//    "tradeMineOne": 0, // 每小时挖矿奖励
//    "totalFeeBtc": 0.00020057, // 交易手续费折合BTC
//    "statTime": "2018-07-31 18:30:19 +0800", // 统计时间
//    "inviteMineOne": 0 // 每小时邀请奖励
//  }
//}
type OneHourlyLimitation struct {
	TradeMineOne  float64 `json:"tradeMineOne"`
	TotalFeeBtc   float64 `json:"totalFeeBtc"`
	StatTime      string  `json:"statTime"`
	InviteMineOne float64 `json:"inviteMineOne"`
}

type OneHourlyLimitationResponeBody struct {
	Data   *OneHourlyLimitation `json:"data"`
	Errors []ErrorMessage       `json:"errors"`
}

type OneLimitationResponeBody struct {
	Data   float64        `json:"data"`
	Errors []ErrorMessage `json:"errors"`
}
