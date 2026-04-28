// Temu 浏览器客户端
package client

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/bestK/temu-web-client/config"
	"github.com/bestK/temu-web-client/entity"
	"github.com/bestK/temu-web-client/log"
	"github.com/bestK/temu-web-client/normal"
	"github.com/bestK/temu-web-client/utils"
	"github.com/go-resty/resty/v2"
	"github.com/goccy/go-json"
)

// service 持有 3 个职责清晰的 resty 客户端：
//   - oAuthClient:         指向 seller.kuajingmaihuo.com，全局账号/OAuth 登录入口（不随 region 变化）
//   - sellerCentralClient: 指向全局默认 Seller Central（agentseller.temu.com），不随 region 变化，
//     用于显式访问 global 卖家中心（即便当前激活了非 global region）
//   - regionClient:        指向当前激活 region 的 Seller Central（agentseller-{region}.temu.com），
//     由 applyRegionConfig 动态切换 BaseURL 与 Cookie，业务接口默认走这里
type service struct {
	debug               bool          // Is debug mode
	logger              resty.Logger  // Log
	oAuthClient         *resty.Client // 登录入口客户端（kuajingmaihuo）
	sellerCentralClient *resty.Client // 全局 Seller Central 客户端（固定 agentseller.temu.com）
	regionClient        *resty.Client // 当前 region 的 Seller Central 客户端（动态）
}

type services struct {
	RecentOrderService           recentOrderService
	BgAuthService                bgAuthService
	StockService                 stockService
	ProductService               productService
	CustomizedInformationService customizedInformationService
	FinanceService               financeService
}

type Client struct {
	Debug                bool
	Logger               resty.Logger
	Services             services
	TimeLocation         *time.Location
	BaseUrl              string
	SellerCentralBaseUrl string
	MallId               int

	// 多 region 支持
	Region  string                         // 当前激活的 region 名
	Regions map[string]config.RegionConfig // 全部 region 配置

	// 底层 resty 客户端，复用同样的 Cookie / Header / 重试 / Anti-Content 配置，
	// 供调用方直接发起未封装的接口请求。通过 OAuthClient() / SellerCentralClient() / RegionClient() 访问。
	oAuthClient         *resty.Client
	sellerCentralClient *resty.Client
	regionClient        *resty.Client
}

// OAuthClient 返回 OAuth/登录入口（seller.kuajingmaihuo.com）的 resty 客户端。
// 可用它直接发起本库尚未封装的 OAuth 域接口，例如：
//
//	resp, err := client.OAuthClient().R().SetBody(params).Post("/bg/quiet/api/xxx")
func (c *Client) OAuthClient() *resty.Client { return c.oAuthClient }

// SellerCentralClient 返回卖家中心主域（agentseller.temu.com）的 resty 客户端。
// 大部分业务接口都在这里，未封装的接口可直接通过它调用：
//
//	resp, err := client.SellerCentralClient().R().
//	    SetHeader("mallid", fmt.Sprintf("%d", client.MallId)).
//	    SetBody(params).
//	    Post("/some/un-wrapped/api")
func (c *Client) SellerCentralClient() *resty.Client { return c.sellerCentralClient }

// RegionClient 返回当前激活 region 的 resty 客户端（agentseller-{region}.temu.com）。
// 仅在配置了 Region 时有意义，未配置时它的 BaseURL 与 SellerCentralClient() 相同但不带 region cookie。
func (c *Client) RegionClient() *resty.Client { return c.regionClient }

// ResponseHook 响应钩子签名，与 resty.ResponseMiddleware 一致。
// hook 在请求收到响应后、result 解析完成后被调用；返回 error 会被作为请求错误抛出。
type ResponseHook func(c *resty.Client, resp *resty.Response) error

// OnResponse 在所有底层 resty 客户端（OAuth/SellerCentral/Region）上注册响应钩子。
// 可多次调用，hook 按注册顺序执行。常用于：审计日志、上报、统一鉴权失效检测等。
//
//	client.OnResponse(func(_ *resty.Client, r *resty.Response) error {
//	    log.Printf("[%d] %s %s", r.StatusCode(), r.Request.Method, r.Request.URL)
//	    return nil
//	})
func (c *Client) OnResponse(hook ResponseHook) *Client {
	if hook == nil {
		return c
	}
	mw := resty.ResponseMiddleware(hook)
	c.oAuthClient.OnAfterResponse(mw)
	c.sellerCentralClient.OnAfterResponse(mw)
	c.regionClient.OnAfterResponse(mw)
	return c
}

// OnError 在所有底层 resty 客户端上注册错误钩子（请求/响应失败时回调）。
//
//	client.OnError(func(req *resty.Request, err error) {
//	    log.Printf("request error: %s %v", req.URL, err)
//	})
func (c *Client) OnError(hook func(req *resty.Request, err error)) *Client {
	if hook == nil {
		return c
	}
	c.oAuthClient.OnError(hook)
	c.sellerCentralClient.OnError(hook)
	c.regionClient.OnError(hook)
	return c
}

func NewClient(config config.TemuBrowserConfig) *Client {
	var logger resty.Logger
	var logLevel = new(slog.LevelVar) // 默认 INFO
	if config.Debug {
		logLevel.Set(slog.LevelDebug)
	}

	opts := &slog.HandlerOptions{
		Level: logLevel,
	}

	// 优先使用调用方传入的 logger；未传入时按 Debug 选择默认 text/json handler
	if config.Logger != nil {
		logger = config.Logger
	} else if config.Debug {
		logger = log.NewSlogAdapter(slog.New(slog.NewTextHandler(os.Stdout, opts)))
	} else {
		logger = log.NewSlogAdapter(slog.New(slog.NewJSONHandler(os.Stdout, opts)))
	}

	client := &Client{
		Debug:                config.Debug,
		BaseUrl:              config.BaseUrl,
		SellerCentralBaseUrl: config.SellerCentralBaseUrl,
		Logger:               logger,
	}

	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		logger.Errorf("load location error: %v", err)
	}
	client.TimeLocation = loc

	oAuthClient := newRestyClient(config, logger, config.BaseUrl)
	sellerCentralClient := newRestyClient(config, logger, config.SellerCentralBaseUrl)
	regionClient := newRestyClient(config, logger, config.SellerCentralBaseUrl)

	client.oAuthClient = oAuthClient
	client.sellerCentralClient = sellerCentralClient
	client.regionClient = regionClient

	xService := service{
		debug:               config.Debug,
		logger:              logger,
		oAuthClient:         oAuthClient,
		sellerCentralClient: sellerCentralClient,
		regionClient:        regionClient,
	}

	// 卖家中心主域 Cookie：应用到 sellerCentralClient
	if config.SellerCentralCookie != "" {
		sellerCentralClient.SetHeader("Cookie", config.SellerCentralCookie)
	}

	// region 专属接口（可选）：启动时应用激活 region 的 Cookie/BaseURL 到 regionClient
	client.Regions = config.Regions
	if config.Region != "" {
		if rc, ok := config.Regions[config.Region]; ok {
			client.Region = config.Region
			applyRegionConfig(client, regionClient, rc)
		}
	}

	client.Services = services{
		RecentOrderService:           recentOrderService{xService, client},
		BgAuthService:                bgAuthService{xService, client},
		StockService:                 stockService{xService, client},
		ProductService:               productService{xService, client},
		CustomizedInformationService: customizedInformationService{xService, client},
		FinanceService:               financeService{xService, client},
	}

	return client
}
func recheckError(resp *resty.Response, result normal.Response, e error) (err error) {
	if e != nil {
		return e
	}

	if resp.IsError() {
		// 对于非2xx响应，手动解析错误信息
		var errorResult normal.Response
		if err := json.Unmarshal(resp.Body(), &errorResult); err != nil {
			return fmt.Errorf("failed to parse error response: %v", err)
		}

		if errorResult.ErrorMessage != "" {
			return normal.GetErrorByCode(errorResult.ErrorCode, errorResult.ErrorMessage).Err
		}

		var errorResult2 normal.Response2
		if err = json.Unmarshal(resp.Body(), &errorResult2); err != nil {
			return fmt.Errorf("failed to parse error response: %v", err)
		}

		if errorResult2.ErrorMessage != "" {
			return normal.GetErrorByCode(errorResult2.ErrorCode, errorResult2.ErrorMessage).Err
		}

		return errors.New("unknown error")
	}

	if !result.Success {
		if result.ErrorCode == entity.ErrorNeedSMSCode {
			return normal.ErrNeedSMSCode
		}

		if result.ErrorCode == entity.ErrorNeedVerifyCode {
			return normal.ErrNeedVerifyCode
		}

		return errors.New(result.ErrorMessage)
	}
	return nil
}

func parseResponseTotal(currentPage, pageSize, total int) (n, totalPages int, isLastPage bool) {
	if currentPage == 0 {
		currentPage = 1
	}

	totalPages = (total / pageSize) + 1
	return total, totalPages, currentPage >= totalPages
}

func (c *Client) SetMallId(mallId int) {
	c.MallId = mallId
}

// newRestyClient 构造一个共用同样配置（debug / 超时 / TLS / Anti-Content / 重试 / 代理）的 resty 客户端。
func newRestyClient(cfg config.TemuBrowserConfig, logger resty.Logger, baseURL string) *resty.Client {
	c := resty.New().
		SetDebug(cfg.Debug).
		EnableTrace().
		SetBaseURL(baseURL).
		SetHeaders(map[string]string{
			"priority":     "u=1, i",
			"pragma":       "no-cache",
			"Content-Type": "application/json",
			"Accept":       "application/json",
			"User-Agent":   "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/147.0.0.0 Safari/537.36",
		}).
		SetAllowGetMethodPayload(true).
		SetTimeout(cfg.Timeout * time.Second).
		SetTransport(&http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: !cfg.VerifySSL},
			DialContext: (&net.Dialer{
				Timeout: cfg.Timeout * time.Second,
			}).DialContext,
		}).
		SetRedirectPolicy(resty.RedirectPolicyFunc(func(req *http.Request, via []*http.Request) error {
			if req.Response.StatusCode == 302 {
				return nil
			}
			return http.ErrUseLastResponse
		})).
		OnBeforeRequest(func(_ *resty.Client, request *resty.Request) error {
			values := make(map[string]any)
			if request.Body != nil {
				b, e := json.Marshal(request.Body)
				if e != nil {
					return e
				}
				if e = json.Unmarshal(b, &values); e != nil {
					return e
				}
			}
			antiContent, err := utils.GetAntiContent()
			if err != nil {
				return err
			}
			request.SetHeader("Anti-Content", antiContent)
			return nil
		}).
		SetRetryCount(3).
		SetRetryWaitTime(time.Duration(500) * time.Millisecond).
		SetRetryMaxWaitTime(time.Duration(1) * time.Second).
		AddRetryCondition(func(response *resty.Response, err error) bool {
			if response == nil {
				return false
			}
			retry := response.StatusCode() == http.StatusTooManyRequests
			if !retry {
				r := struct {
					Success   bool   `json:"success"`
					ErrorCode int    `json:"errorCode"`
					ErrorMsg  string `json:"errorMsg"`
				}{}
				retry = json.Unmarshal(response.Body(), &r) == nil &&
					!r.Success &&
					r.ErrorCode == 4000000 &&
					strings.EqualFold(r.ErrorMsg, "SYSTEM_EXCEPTION")
			}
			if retry {
				antiContent, err := utils.GetAntiContent()
				if err != nil {
					logger.Errorf("重新获取 Anti-Content 失败: %v", err)
					return false
				}
				response.Request.SetHeader("Anti-Content", antiContent)
				logger.Debugf("重试请求，URL: %s", response.Request.URL)
			}
			return retry
		})

	if cfg.Proxy != "" {
		c.SetProxy(cfg.Proxy)
	}
	c.JSONMarshal = json.Marshal
	c.JSONUnmarshal = json.Unmarshal
	return c
}

// applyRegionConfig 把单个 region 的 Cookie / BaseURL 应用到 regionClient。
// 不影响 oAuthClient 与 sellerCentralClient（这两个是固定全局域名）。
func applyRegionConfig(_ *Client, regionClient *resty.Client, rc config.RegionConfig) {
	if rc.BaseUrl != "" {
		regionClient.SetBaseURL(rc.BaseUrl)
	}
	if rc.Cookie != "" {
		regionClient.SetHeader("Cookie", rc.Cookie)
	}
}

// UseRegion 切换到指定 region；该 region 必须已经在 Regions 中注册
func (c *Client) UseRegion(region string) error {
	rc, ok := c.Regions[region]
	if !ok {
		return fmt.Errorf("region %q 未在配置中注册", region)
	}
	c.Region = region
	applyRegionConfig(c, c.Services.BgAuthService.regionClient, rc)
	return nil
}

// RegisterRegion 动态注册或覆盖一个 region 的 Cookie 配置
func (c *Client) RegisterRegion(region string, rc config.RegionConfig) {
	if c.Regions == nil {
		c.Regions = map[string]config.RegionConfig{}
	}
	c.Regions[region] = rc
}

func (c *Client) CheckMallId() error {
	if c.MallId == 0 {
		return errors.New("mall ID is not set")
	}
	return nil
}

func (c *Client) IsAccountSessionInvalid() bool {
	_, err := c.Services.BgAuthService.GetAccountUserInfo(context.Background())
	return err != nil
}

func (c *Client) IsSellerCentralSessionInvalid() bool {
	_, err := c.Services.BgAuthService.GetSellerCentralUserInfo(context.Background())
	return err != nil
}
