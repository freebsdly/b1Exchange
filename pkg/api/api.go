package api

import (
	"b1Exchange/pkg/model"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/json-iterator/go"
)

var (
	json = jsoniter.ConfigCompatibleWithStandardLibrary
)

type Client struct {
	endPoint  string
	appKey    string
	appSecret []byte

	httpClient *http.Client
}

// 创建b1的api客户端
func NewClient(ep, key, secret string, timeout int64) *Client {
	var tp = &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	return &Client{
		endPoint:  ep,
		appKey:    key,
		appSecret: []byte(secret),
		httpClient: &http.Client{
			Transport: tp,
			Timeout:   time.Duration(timeout) * time.Millisecond,
		},
	}
}

// ping返回时间戳
// {
//   "timestamp": 1527665262168391000
// }
func (p *Client) Ping() (ts int64, err error) {
	resp, err := p.httpClient.Get(fmt.Sprintf("%s/%s", p.endPoint, "ping"))
	if err != nil {
		return -1, err
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return -1, err
	}
	defer resp.Body.Close()

	var body = new(model.PingResponeBody)
	err = json.Unmarshal(data, body)
	if err != nil {
		return -1, fmt.Errorf("unmarshal failed. data: %s", string(data))
	}

	return body.Timestamp, nil

}

// 获取所有的市场，即交易对
func (p *Client) GetAllMarkets() (*model.MarketResponeBody, error) {
	resp, err := p.httpClient.Get(fmt.Sprintf("%s/%s", p.endPoint, "markets"))
	if err != nil {
		return nil, err
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var body = new(model.MarketResponeBody)
	err = json.Unmarshal(data, body)
	if err != nil {
		return nil, fmt.Errorf("unmarshal failed. data: %s", string(data))
	}

	if len(body.Errors) != 0 {
		return nil, fmt.Errorf("get all markets respone have errors. data: %s", string(data))
	}

	return body, nil

}

// 获取总体行情信息
// 由于bigone支持较多的交易对，一次获取所有行情数据比较多
// GET /tickers
func (p *Client) GetAllTickers() (*model.AllTickersResponeBody, error) {
	resp, err := p.httpClient.Get(fmt.Sprintf("%s/%s", p.endPoint, "tickers"))
	if err != nil {
		return nil, err
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var body = new(model.AllTickersResponeBody)
	err = json.Unmarshal(data, body)
	if err != nil {
		return nil, fmt.Errorf("unmarshal failed. data: %s", string(data))
	}

	if len(body.Errors) != 0 {
		return nil, fmt.Errorf("get all tickers respone have errors. data: %s", string(data))
	}

	return body, nil
}

// 获取单个市场行情
// GET /markets/{market_id}/ticker
// market_id: ETH-BTC
func (p *Client) GetTicker(id string) (*model.MarketTickerResponeBody, error) {
	resp, err := p.httpClient.Get(fmt.Sprintf("%s/markets/%s/%s", p.endPoint, id, "ticker"))
	if err != nil {
		return nil, err
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var body = new(model.MarketTickerResponeBody)
	err = json.Unmarshal(data, body)
	if err != nil {
		return nil, fmt.Errorf("unmarshal failed. data: %s", string(data))
	}

	if len(body.Errors) != 0 {
		return nil, fmt.Errorf("get ticker respone have errors. data: %s", string(data))
	}

	return body, nil
}

// jwt 签名
// 由于nonce只能使用一次，使用go同步会导致取到的unixnano相同，导致其中一个订单创建失败
// 所有这里使用入参nonce，调用时可以手工+1使得nonce不一样
func (p *Client) JWTSignature(nonce int64) (string, error) {
	claims := make(jwt.MapClaims)
	claims["type"] = "OpenAPI"
	claims["sub"] = p.appKey
	claims["nonce"] = nonce

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(p.appSecret)
}

// 获取账户资产信息
// GET /viewer/accounts
func (p *Client) GetAccounts(nonce int64) (*model.AccountResponeBody, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/%s", p.endPoint, "viewer/accounts"), nil)
	if err != nil {
		return nil, err
	}

	token, err := p.JWTSignature(nonce)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var body = new(model.AccountResponeBody)
	err = json.Unmarshal(data, body)
	if err != nil {
		return nil, fmt.Errorf("unmarshal failed. data: %s", string(data))
	}

	if len(body.Errors) != 0 {
		return nil, fmt.Errorf("get account respone have errors. data: %s", string(data))
	}

	return body, nil
}

//
// market_id market id ETH-BTC true
// after ask for the server to return orders after the cursor dGVzdGN1cmVzZQo= false
// before ask for the server to return orders before the cursor dGVzdGN1cmVzZQo= false
// first slicing count 20 false
// last slicing count 20 false
// side order side one of "ASK"/"BID" false
// state order state one of "CANCELED"/"FILLED"/"PENDING" false
func (p *Client) GetOrders(nonce int64, parms map[string]string) (*model.OrderListResponeBody, error) {
	reqUrl, err := url.Parse(fmt.Sprintf("%s/%s", p.endPoint, "viewer/orders"))
	if err != nil {
		return nil, err
	}
	query := reqUrl.Query()
	for k, v := range parms {
		query.Add(k, v)
	}

	reqUrl.RawQuery = query.Encode()
	req, err := http.NewRequest("GET", reqUrl.String(), nil)
	if err != nil {
		return nil, err
	}

	token, err := p.JWTSignature(nonce)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var body = new(model.OrderListResponeBody)
	err = json.Unmarshal(data, body)
	if err != nil {
		return nil, fmt.Errorf("unmarshal failed. data: %s", string(data))
	}

	if len(body.Errors) != 0 {
		return nil, fmt.Errorf("get orders respone have errors. data: %s", string(data))
	}

	return body, nil
}

// POST /viewer/orders
func (p *Client) CreateOrder(nonce int64, parms map[string]string) (*model.Order, error) {
	reqUrl, err := url.Parse(fmt.Sprintf("%s/%s", p.endPoint, "viewer/orders"))
	if err != nil {
		return nil, err
	}
	query := reqUrl.Query()
	for k, v := range parms {
		query.Add(k, v)
	}

	reqUrl.RawQuery = query.Encode()
	req, err := http.NewRequest("POST", reqUrl.String(), nil)
	if err != nil {
		return nil, err
	}

	token, err := p.JWTSignature(nonce)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var body = new(model.OrderResponeBody)
	err = json.Unmarshal(data, body)
	if err != nil {
		return nil, fmt.Errorf("unmarshal failed. data: %s", string(data))
	}

	if len(body.Errors) != 0 {
		return nil, fmt.Errorf("create order respone have errors. data: %s", string(data))
	}

	return body.Data, nil
}

// POST /viewer/orders/{order_id}/cancel
func (p *Client) CancelOrder(nonce int64, id string) (*model.Order, error) {
	reqUrl, err := url.Parse(fmt.Sprintf("%s/%s/%s/cancel", p.endPoint, "viewer/orders", id))
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", reqUrl.String(), nil)
	if err != nil {
		return nil, err
	}

	token, err := p.JWTSignature(nonce)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var body = new(model.OrderResponeBody)
	err = json.Unmarshal(data, body)
	if err != nil {
		return nil, fmt.Errorf("unmarshal failed. data: %s", string(data))
	}

	if len(body.Errors) != 0 {
		return nil, fmt.Errorf("cancel order result have errors. data: %s", string(data))
	}

	return body.Data, nil
}

// POST /viewer/orders/cancel_all
func (p *Client) CancelAllOrders(nonce int64, market string) error {
	reqUrl, err := url.Parse(fmt.Sprintf("%s/%s/cancel_all", p.endPoint, "viewer/orders"))
	if err != nil {
		return err
	}
	query := reqUrl.Query()
	query.Add("market_id", market)

	reqUrl.RawQuery = query.Encode()
	req, err := http.NewRequest("POST", reqUrl.String(), nil)
	if err != nil {
		return err
	}

	token, err := p.JWTSignature(nonce)
	if err != nil {
		return err
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return err
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var body = new(model.Order)
	err = json.Unmarshal(data, body)
	if err != nil {
		return fmt.Errorf("unmarshal failed. data: %s", string(data))
	}

	return nil
}

// 获取每天挖矿限量
// Get One daily limitation
// 每天挖矿上限
// GET /one/limitation
// {
//   "data": 80000000
// }
func (p *Client) OneHourlyStatistic() (*model.OneHourlyLimitationResponeBody, error) {
	resp, err := p.httpClient.Get(fmt.Sprintf("%s/%s", p.endPoint, "one"))
	if err != nil {
		return nil, err
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var body = new(model.OneHourlyLimitationResponeBody)
	err = json.Unmarshal(data, body)
	if err != nil {
		return nil, fmt.Errorf("unmarshal failed. %s, data: %s", err, string(data))
	}

	if len(body.Errors) != 0 {
		return nil, fmt.Errorf("get one hourly statistic respone have errors. data: %s", string(data))
	}

	return body, nil
}

func (p *Client) OneLimitation() (*model.OneLimitationResponeBody, error) {
	resp, err := p.httpClient.Get(fmt.Sprintf("%s/%s", p.endPoint, "one/limitation"))
	if err != nil {
		return nil, err
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var body = new(model.OneLimitationResponeBody)
	err = json.Unmarshal(data, body)
	if err != nil {
		return nil, fmt.Errorf("unmarshal failed. data: %s", string(data))
	}

	if len(body.Errors) != 0 {
		return nil, fmt.Errorf("get one limitation respone have errors. data: %s", string(data))
	}

	return body, nil
}
