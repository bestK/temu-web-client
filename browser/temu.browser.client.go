package browser

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/bestk/temu-helper/config"
	"github.com/bestk/temu-helper/normal"
	"github.com/bestk/temu-helper/utils"
	"github.com/go-resty/resty/v2"
)

type service struct {
	debug      bool          // Is debug mode
	logger     *log.Logger   // Log
	httpClient *resty.Client // HTTP client
}

type services struct {
	BgOrderService bgOrderService
	BgAuthService  bgAuthService
}

type Client struct {
	Debug                bool           // Is debug mode
	Logger               *log.Logger    // Log
	Services             services       // API services
	TimeLocation         *time.Location // Time location
	BaseUrl              string         // Base URL
	SellerCentralBaseUrl string         // Seller Central Base URL
}

func New(config config.TemuBrowserConfig) *Client {
	logger := log.New(os.Stdout, "[ Temu ] ", log.LstdFlags|log.Llongfile)
	client := &Client{
		Debug:                config.Debug,
		Logger:               logger,
		BaseUrl:              config.BaseUrl,
		SellerCentralBaseUrl: config.SellerCentralBaseUrl,
	}
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		logger.Println("load location error:", err)
	}
	client.TimeLocation = loc

	httpClient := resty.New().
		SetDebug(config.Debug).
		EnableTrace().
		SetBaseURL(config.BaseUrl).
		SetHeaders(map[string]string{
			"Content-Type": "application/json",
			"Accept":       "application/json",
			"User-Agent":   config.UserAgent,
		}).
		SetAllowGetMethodPayload(true).
		SetTimeout(config.Timeout * time.Second).
		SetTransport(&http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: !config.VerifySSL},
			DialContext: (&net.Dialer{
				Timeout: config.Timeout * time.Second,
			}).DialContext,
		}).
		SetRedirectPolicy(resty.RedirectPolicyFunc(func(req *http.Request, via []*http.Request) error {
			if req.Response.StatusCode == 302 {
				return nil
			}
			return http.ErrUseLastResponse
		})).
		OnBeforeRequest(func(client *resty.Client, request *resty.Request) error {
			values := make(map[string]any)
			if request.Body != nil {
				b, e := json.Marshal(request.Body)
				if e != nil {
					return e
				}

				e = json.Unmarshal(b, &values)
				if e != nil {
					return e
				}
			}
			// 设置请求头中的Anti-Content
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
				// 重新设置 Anti-Content
				antiContent, err := utils.GetAntiContent()
				if err != nil {
					logger.Printf("重新获取 Anti-Content 失败: %v", err)
					return false
				}
				response.Request.SetHeader("Anti-Content", antiContent)

				logger.Printf("重试请求，URL: %s", response.Request.URL)
			}
			return retry
		})
	if config.Proxy != "" {
		httpClient.SetProxy(config.Proxy)
	}
	httpClient.JSONMarshal = json.Marshal
	httpClient.JSONUnmarshal = json.Unmarshal
	xService := service{
		debug:      config.Debug,
		logger:     logger,
		httpClient: httpClient,
	}
	client.Services = services{
		BgOrderService: bgOrderService{xService, client},
		BgAuthService:  bgAuthService{xService, client},
	}

	return client
}

func recheckError(resp *resty.Response, result normal.Response, e error) (err error) {
	if e != nil {
		return e
	}

	if resp.IsError() {
		errorMessage := strings.TrimSpace(result.ErrorMessage)

		return errors.New(errorMessage)
	}

	if !result.Success {
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
