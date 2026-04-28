package config

import (
	"time"

	"github.com/go-resty/resty/v2"
)

// 预定义 Region 名称，使用方也可以自定义任意名称
const (
	RegionDefault = "default"
	RegionUS      = "us"
	RegionEU      = "eu"
	RegionGlobal  = "global"
)

// RegionConfig 描述一个 region 下 *region 专属* 接口的鉴权与域名。
//
// 客户端职责划分：
//   - oAuthClient        → seller.kuajingmaihuo.com   登录/账号入口（不随 region 变化）
//   - sellerCentralClient → agentseller.temu.com       卖家中心主业务域，承载绝大多数业务 API（不随 region 变化）
//   - regionClient        → agentseller-{region}.temu.com  region 专属域名，用于该 region 独有接口
//
// 因此：
//   - Seller Central 的 Cookie 在顶层 TemuBrowserConfig.SellerCentralCookie 里配置；
//   - RegionConfig.Cookie 仅作用于 regionClient（region 专属域名上的会话）。
//
// MallId 不放在配置里，由调用方在需要时通过 Client.SetMallId 传入。
type RegionConfig struct {
	Cookie  string `json:"cookie"`             // 当前 region 自有域名上的会话 Cookie
	BaseUrl string `json:"base_url,omitempty"` // 当前 region 的专属域名，例如 https://agentseller-eu.temu.com
}

type TemuBrowserConfig struct {
	Debug                bool          `json:"debug"`                                                                                                                                      // 是否为调试模式
	BaseUrl              string        `json:"base_url"`                                                                                                                                   // OAuth/登录入口（kuajingmaihuo）基础 URL
	SellerCentralBaseUrl string        `json:"seller_central_base_url"`                                                                                                                    // 卖家中心主业务域基础 URL（agentseller.temu.com）
	SellerCentralCookie  string        `json:"seller_central_cookie,omitempty"`                                                                                                            // 卖家中心 Cookie，应用到 sellerCentralClient
	Timeout              time.Duration `json:"timeout"`                                                                                                                                    // 超时时间（秒）
	VerifySSL            bool          `json:"verify_ssl"`                                                                                                                                 // 是否验证 SSL
	UserAgent            string        `json:"user_agent" default:"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"` // User Agent
	Proxy                string        `json:"proxy"`                                                                                                                                      // 代理

	// region 专属接口支持（可选）
	Region  string                  `json:"region,omitempty"`  // 启动时激活的 region；为空则不启用 regionClient 覆盖
	Regions map[string]RegionConfig `json:"regions,omitempty"` // 各 region 自身域名 + Cookie

	Logger resty.Logger `json:"-"`
}
