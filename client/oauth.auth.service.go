// 基础认证服务
package client

import (
	"context"
	"fmt"
	"net/http"

	"github.com/bestK/temu-web-client/entity"
	"github.com/bestK/temu-web-client/normal"
	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/goccy/go-json"
)

type bgAuthService struct {
	service
	client *Client
}

type BgLoginRequestParams struct {
	LoginName       string  `json:"loginName" binding:"required"`
	EncryptPassword string  `json:"encryptPassword" binding:"required"`
	KeyVersion      string  `json:"keyVersion" default:"1" binding:"required"`
	VerifyCode      *string `json:"verifyCode"`
}

type BgObtainCodeRequestParams struct {
	RedirectUrl string `json:"redirectUrl" binding:"required"`
}

type BgLoginByCodeRequestParams struct {
	Code         string `json:"code" binding:"required"`
	Confirm      bool   `json:"confirm" default:"false"`
	TargetMallId int    `json:"targetMallId" binding:"required"`
}

type BgGetLoginVerifyCodeRequestParams struct {
	Mobile string `json:"mobile" binding:"required"`
}

func (m BgLoginRequestParams) validate() error {
	return validation.ValidateStruct(&m,
		validation.Field(&m.LoginName, validation.Required.Error("登录名不能为空")),
		validation.Field(&m.EncryptPassword, validation.Required.Error("加密密码不能为空")),
	)
}

func (m BgObtainCodeRequestParams) validate() error {
	return validation.ValidateStruct(&m,
		validation.Field(&m.RedirectUrl, validation.Required.Error("重定向URL不能为空")),
	)
}

func (m BgLoginByCodeRequestParams) validate() error {
	return validation.ValidateStruct(&m,
		validation.Field(&m.Code, validation.Required.Error("验证码不能为空")),
	)
}

func (m BgGetLoginVerifyCodeRequestParams) validate() error {
	return validation.ValidateStruct(&m,
		validation.Field(&m.Mobile, validation.Required.Error("手机号不能为空")),
	)
}

func (s *bgAuthService) GetPublicKey() (string, string, error) {
	var result = struct {
		normal.Response `json:",inline"`
		Result          struct {
			PublicKey string `json:"publicKey"`
			Version   string `json:"version"`
		} `json:"result"`
	}{}

	resp, err := s.oAuthClient.R().
		SetResult(&result).
		Post("/bg/quiet/api/mms/key/login")

	if err != nil {
		return "", "", err
	}

	err = json.Unmarshal(resp.Body(), &result)
	if err != nil {
		return "", "", err
	}

	if err = recheckError(resp, result.Response, err); err != nil {
		s.client.Logger.Errorf("获取公钥失败: %v %+v", err, string(resp.Body()))
		return "", "", err
	}

	return result.Result.PublicKey, result.Result.Version, nil
}

// https://seller.kuajingmaihuo.com/bg/quiet/api/mms/login
func (s *bgAuthService) Login(ctx context.Context, params BgLoginRequestParams) (int, []*http.Cookie, error) {
	if err := params.validate(); err != nil {
		return 0, nil, err
	}

	var result = struct {
		normal.Response `json:",inline"`
		Result          struct {
			MaskMobile      string `json:"maskMobile"`
			VerifyAuthToken string `json:"verifyAuthToken"`
			AccountId       int    `json:"accountId"`
		} `json:"result"`
	}{}

	resp, err := s.oAuthClient.R().
		SetContext(ctx).
		SetResult(&result).
		SetBody(params).
		Post("/bg/quiet/api/mms/login")

	if err = recheckError(resp, result.Response, err); err != nil {
		s.client.Logger.Errorf("登录失败: %v %+v", err, string(resp.Body()))
		return 0, nil, err
	}

	return result.Result.AccountId, resp.Cookies(), nil
}

// ObtainCode 获取验证码 bg/quiet/api/auth/obtainCode
func (s *bgAuthService) ObtainCode(ctx context.Context, params BgObtainCodeRequestParams) (string, error) {
	if err := params.validate(); err != nil {
		return "", err
	}

	var result = struct {
		normal.Response `json:",inline"`
		Result          struct {
			Code string `json:"code"`
		} `json:"result"`
	}{}

	resp, err := s.oAuthClient.R().
		SetContext(ctx).
		SetResult(&result).
		SetBody(params).
		Post("/bg/quiet/api/auth/obtainCode")

	if err = recheckError(resp, result.Response, err); err != nil {
		s.client.Logger.Errorf("获取验证码失败: %v %+v", err, string(resp.Body()))
		return "", err
	}

	return result.Result.Code, nil
}

// LoginTemuAccount 登录 Temu 账号
func (s *bgAuthService) LoginTemuAccount(ctx context.Context, url string) (string, []*http.Cookie, error) {
	resp, err := s.oAuthClient.R().
		SetContext(ctx).
		SetDoNotParseResponse(true).
		Get(url)
	if err != nil {
		s.client.Logger.Errorf("登录 Temu 账号失败: %v %+v", err, string(resp.Body()))
		return "", nil, err
	}
	defer resp.RawResponse.Body.Close()
	return resp.String(), resp.Cookies(), nil
}

// api/seller/auth/loginByCode
func (s *bgAuthService) LoginSellerCentralByCode(ctx context.Context, params BgLoginByCodeRequestParams) (bool, []*http.Cookie, error) {
	if err := params.validate(); err != nil {
		return false, nil, err
	}

	var result = struct {
		normal.Response
		Result struct {
			VerifyAuthToken string `json:"verifyAuthToken"`
		} `json:"result"`
	}{}

	resp, err := s.sellerCentralClient.R().
		SetContext(ctx).
		SetResult(&result).
		SetBody(params).
		Post("/api/seller/auth/loginByCode")

	if err = recheckError(resp, result.Response, err); err != nil {
		s.client.Logger.Errorf("登录 Seller Central 失败: %v %+v", err, string(resp.Body()))
		return false, nil, err
	}

	return true, resp.Cookies(), nil
}

// 获取登录短信验证码 bg/quiet/api/mms/loginVerifyCode
func (s *bgAuthService) GetLoginVerifyCode(ctx context.Context, params BgGetLoginVerifyCodeRequestParams) (bool, error) {
	if err := params.validate(); err != nil {
		return false, err
	}

	var result = struct {
		normal.Response `json:",inline"`
	}{}

	resp, err := s.oAuthClient.R().
		SetContext(ctx).
		SetResult(&result).
		SetBody(params).
		Post("/bg/quiet/api/mms/loginVerifyCode")

	if err != nil {
		return false, err
	}

	if err = recheckError(resp, result.Response, err); err != nil {
		s.client.Logger.Errorf("获取登录短信验证码失败: %v %+v", err, string(resp.Body()))
		return false, err
	}

	return true, nil
}

// WhoAmI 通过已配置的 Cookie 验证当前会话身份并返回用户信息。
// 业务接口走 sellerCentralClient，所需 Cookie 在 TemuBrowserConfig.SellerCentralCookie 中配置。
// 如传入非空 region，会先切换 regionClient 的 BaseURL/Cookie 再做校验。
//
// 副作用：自动把返回 MallList 中的第一个 MallId 设到 Client 上（SetMallId），
// 后续业务接口无需调用方再单独 SetMallId。若账号下有多个 mall，会打 WARN 日志，
// 调用方可显式 SetMallId 覆盖。
func (s *bgAuthService) WhoAmI(ctx context.Context, region string) (entity.UserInfo, error) {
	if region != "" {
		if err := s.client.UseRegion(region); err != nil {
			return entity.UserInfo{}, err
		}
	}
	userInfo, err := s.GetSellerCentralUserInfo(ctx)
	if err != nil {
		return entity.UserInfo{}, fmt.Errorf("Cookie 身份校验失败: %w", err)
	}

	if n := len(userInfo.MallList); n > 0 {
		first := userInfo.MallList[0]
		s.client.SetMallId(first.MallId)
		if n > 1 {
			s.client.Logger.Warnf("账号下存在 %d 个 mall，默认使用第一个 mallId=%d (%s)；如需切换请显式调用 SetMallId",
				n, first.MallId, first.MallName)
		}
	}
	return userInfo, nil
}

// // 获取用户信息 api/seller/auth/userInfo
func (s *bgAuthService) GetSellerCentralUserInfo(ctx context.Context) (entity.UserInfo, error) {
	var result = struct {
		normal.Response
		Result entity.UserInfo `json:"result"`
	}{}

	resp, err := s.sellerCentralClient.R().
		SetContext(ctx).
		SetHeader("Origin", "https://agentseller.temu.com").
		SetBody(map[string]interface{}{}).
		SetResult(&result).
		Post("/api/seller/auth/userInfo")

	if err != nil {
		return entity.UserInfo{}, err
	}

	if err = recheckError(resp, result.Response, err); err != nil {
		s.client.Logger.Errorf("获取用户信息失败: %v %+v", err, string(resp.Body()))
		return entity.UserInfo{}, err
	}

	return result.Result, nil
}

// 获取用户信息 https://seller.kuajingmaihuo.com/bg/quiet/api/mms/userInfo
func (s *bgAuthService) GetAccountUserInfo(ctx context.Context) ([]entity.AccountMallInfo, error) {
	var result = struct {
		normal.Response `json:",inline"`
		Result          struct {
			CompanyList []struct {
				MalInfoList []entity.AccountMallInfo `json:"malInfoList"`
			} `json:"companyList"`
		} `json:"result"`
	}{}

	resp, err := s.oAuthClient.R().
		SetContext(ctx).
		SetResult(&result).
		SetBody(map[string]interface{}{}).
		Post("/bg/quiet/api/mms/userInfo")

	if err != nil {
		return []entity.AccountMallInfo{}, err
	}

	if err = recheckError(resp, result.Response, err); err != nil {
		s.client.Logger.Errorf("获取用户信息失败: %v %+v", err, string(resp.Body()))
		return []entity.AccountMallInfo{}, err
	}

	return result.Result.CompanyList[0].MalInfoList, nil
}
