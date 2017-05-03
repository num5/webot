package webot

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"github.com/num5/logger"
	"io"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const httpOK = `200`

// BaseRequest is a base for all wx api request.
type BaseRequest struct {
	XMLName xml.Name `xml:"error" json:"-"`

	Ret        int    `xml:"ret" json:"-"`
	Message    string `xml:"message" json:"-"`
	Wxsid      string `xml:"wxsid" json:"Sid"`
	Skey       string `xml:"skey"`
	DeviceID   string `xml:"-"`
	Wxuin      int64  `xml:"wxuin" json:"Uin"`
	PassTicket string `xml:"pass_ticket" json:"-"`
}

// Caller is a interface, All response need implement this.
type Caller interface {
	IsSuccess() bool
	Error() error
}

// Response is a wrapper.
type Response struct {
	BaseResponse *BaseResponse
}

// IsSuccess flag this request is success or failed.
func (response *Response) IsSuccess() bool {
	return response.BaseResponse.Ret == 0
}

// response's error msg.
func (response *Response) Error() error {
	return fmt.Errorf("错误信息: %s", response.BaseResponse.ErrMsg)
}

// BaseResponse for all api resp.
type BaseResponse struct {
	Ret    int
	ErrMsg string
}

// Configure ...
type Configure struct {
	Processor UUIDProcessor
	Debug     bool
	Storage   string
	version   string
}

// DefaultConfigure create default configuration
func DefaultConfigure() *Configure {
	return &Configure{
		Processor: new(defaultUUIDProcessor),
		Debug:     false,
		Storage:   `.webot`,
		version:   `1.0.0-rc1`,
	}
}

func NewConfigure(processor UUIDProcessor, debug bool, storpath string, version string) *Configure {
	return &Configure{
		Processor: processor,
		Debug:     debug,
		Storage:   storpath,
		version:   version,
	}
}

func (c *Configure) contactCachePath() string {
	return filepath.Join(c.Storage, `contact-cache.json`)
}
func (c *Configure) baseInfoCachePath() string {
	return filepath.Join(c.Storage, `basic-info-cache.json`)
}
func (c *Configure) cookieCachePath() string {
	return filepath.Join(c.Storage, `cookie-cache.json`)
}

func (c *Configure) httpStoragePath(url *url.URL) string {
	ps := strings.Split(url.Path, `/`)
	lastP := strings.Split(ps[len(ps)-1], `?`)[0][5:]
	return c.Storage + `/` + lastP
}

// WeChat container a default http client and base request.
type WeChat struct {
	Client      *http.Client
	BaseURL     string
	BaseRequest *BaseRequest
	MySelf      Contact
	IsLogin     bool

	conf       *Configure
	evtStream  *evtStream
	cache      *cache
	syncKey    map[string]interface{}
	syncHost   string
	retryTimes time.Duration
	loginState chan int // -1 登录失败 1登录成功
}

// NewWeChat is desined for Create a new Wechat instance.
func newWeChat(conf *Configure) (*WeChat, error) {

	if _, err := os.Stat(conf.Storage); err != nil {
		if os.IsNotExist(err) {
			err = os.MkdirAll(conf.Storage, os.ModePerm)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	client, err := newClient()
	if err != nil {
		return nil, err
	}

	rand.Seed(time.Now().Unix())
	str := strconv.Itoa(rand.Int())
	device_id := "e" + str[2:17]

	baseReq := new(BaseRequest)
	baseReq.Ret = 1
	baseReq.DeviceID = device_id

	wechat := &WeChat{
		Client:      client,
		BaseRequest: baseReq,
		evtStream:   newEvtStream(),
		IsLogin:     false,
		retryTimes:  time.Duration(0),
		loginState:  make(chan int),
		conf:        conf,
		cache:       newCache(conf.contactCachePath()),
	}

	return wechat, nil
}

func newClient() (*http.Client, error) {

	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}

	transport := http.Transport{
		Dial: (&net.Dialer{
			Timeout: 1 * time.Minute,
		}).Dial,
		TLSHandshakeTimeout: 1 * time.Minute,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}

	client := &http.Client{
		Transport: &transport,
		Jar:       jar,
		Timeout:   1 * time.Minute,
	}

	return client, nil
}

// AwakenNewBot is start point for wx bot.
func AwakenNewBot(conf *Configure) (*WeChat, error) {

	if conf == nil {
		conf = DefaultConfigure()
	}

	wechat, err := newWeChat(conf)

	if err != nil {
		return nil, err
	}

	wechat.evtStream.init()
	go func() {
		for {
			ls := <-wechat.loginState
			event := Event{
				Path: `/login`,
				From: `Wechat`,
				To:   `End`,
				Data: ls,
				Time: time.Now().Unix(),
			}
			wechat.evtStream.serverEvt <- event
		}
	}()

	wechat.keepAlive()

	return wechat, nil
}

func (wechat *WeChat) SetProcessor(processor UUIDProcessor) {
	wechat.conf.Processor = processor
}

func (wechat *WeChat) SetDebug(debug bool) {
	wechat.conf.Debug = debug
}

func (wechat *WeChat) SetStorage(storpath string) {
	wechat.conf.Storage = storpath
}

// ExcuteRequest is desined for perform http request
func (wechat *WeChat) ExcuteRequest(req *http.Request, call Caller) error {

	filename := wechat.conf.httpStoragePath(req.URL)

	if wechat.conf.Debug {
		reqData, _ := httputil.DumpRequestOut(req, false)
		createFile(filename+`_req.json`, reqData, false)
		c, _ := json.Marshal(wechat.Client.Jar.Cookies(req.URL))
		createFile(filename+`_req.json`, c, true)
	}

	resp, err := wechat.Client.Do(req)

	if err != nil {
		return err
	}
	defer resp.Body.Close()

	reader := resp.Body.(io.Reader)

	if wechat.conf.Debug {

		data, e := ioutil.ReadAll(reader)
		if e != nil {
			return e
		}

		createFile(filename+`_resp.json`, data, true)
		reader = bytes.NewReader(data)
	}

	if err = json.NewDecoder(reader).Decode(call); err != nil {
		return err
	}

	if !call.IsSuccess() {
		return call.Error()
	}

	wechat.refreshBaseInfo()
	wechat.refreshCookieCache(resp.Cookies())

	return nil
}

// Excute a http request by default http client.
func (wechat *WeChat) Excute(path string, body io.Reader, call Caller) error {
	method := "GET"
	if body != nil {
		method = "POST"
	}
	req, err := http.NewRequest(method, path, body)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json; charset=UTF-8")
	req.Header.Set(`User-Agent`, `Mozilla/5.0 (Macintosh; Intel Mac OS X 10_12_2) AppleWebKit/602.3.12 (KHTML, like Gecko) Version/10.0.2 Safari/602.3.12`)

	return wechat.ExcuteRequest(req, call)
}

// PassTicketKV return a string like `pass_ticket=sdfewsvdwd=`
func (wechat *WeChat) PassTicketKV() string {
	return fmt.Sprintf(`pass_ticket=%s`, wechat.BaseRequest.PassTicket)
}

// SkeyKV return a string like `skey=ewfwoefjwofjskfwes`
func (wechat *WeChat) SkeyKV() string {
	return fmt.Sprintf(`skey=%s`, wechat.BaseRequest.Skey)
}

var log *logger.Log

func init() {

	// 初始化
	log = logger.NewLog(1000)

	// 设置log级别
	log.SetLevel("Debug")

	// 设置输出引擎
	log.SetEngine("file", `{"level":4, "spilt":"size", "filename":".logs/wechat.log", "maxsize":10}`)

	//log.DelEngine("console")

	// 设置是否输出行号
	log.SetFuncCall(true)
}
