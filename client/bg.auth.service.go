// 基础认证服务
package client

import (
	"context"
	"encoding/json"

	"github.com/bestk/temu-helper/entity"
	"github.com/bestk/temu-helper/normal"
	validation "github.com/go-ozzo/ozzo-validation/v4"
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
		normal.ResponseKuajingmaihuo
		Result struct {
			PublicKey string `json:"publicKey"`
			Version   string `json:"version"`
		} `json:"result"`
	}{}

	resp, err := s.httpClient.R().
		SetResult(&result).
		Post("/bg/quiet/api/mms/key/login")

	if err != nil {
		return "", "", err
	}

	err = json.Unmarshal(resp.Body(), &result)
	if err != nil {
		return "", "", err
	}

	err = recheckErrorKuajingmaihuo(resp, result.ResponseKuajingmaihuo, err)
	if err != nil {
		return "", "", err
	}

	return result.Result.PublicKey, result.Result.Version, nil
}

func (s *bgAuthService) Login(ctx context.Context, params BgLoginRequestParams) (int, error) {
	if err := params.validate(); err != nil {
		return 0, err
	}

	var result = struct {
		normal.ResponseKuajingmaihuo
		Result struct {
			MaskMobile      string `json:"maskMobile"`
			VerifyAuthToken string `json:"verifyAuthToken"`
			AccountId       int    `json:"accountId"`
		} `json:"result"`
	}{}

	resp, err := s.httpClient.R().
		SetContext(ctx).
		SetResult(&result).
		SetBody(params).
		Post("/bg/quiet/api/mms/login")

	if err != nil {
		return 0, err
	}

	err = recheckErrorKuajingmaihuo(resp, result.ResponseKuajingmaihuo, err)
	if err != nil {
		return 0, err
	}

	return result.Result.AccountId, nil
}

// ObtainCode 获取验证码 bg/quiet/api/auth/obtainCode
func (s *bgAuthService) ObtainCode(ctx context.Context, params BgObtainCodeRequestParams) (string, error) {
	if err := params.validate(); err != nil {
		return "", err
	}

	var result = struct {
		normal.ResponseKuajingmaihuo
		Result struct {
			Code string `json:"code"`
		} `json:"result"`
	}{}

	resp, err := s.httpClient.R().
		SetContext(ctx).
		SetResult(&result).
		SetBody(params).
		Post("/bg/quiet/api/auth/obtainCode")

	if err != nil {
		return "", err
	}

	err = recheckErrorKuajingmaihuo(resp, result.ResponseKuajingmaihuo, err)
	if err != nil {
		return "", err
	}

	return result.Result.Code, nil
}

// LoginSellerCentral 登录 Seller Central
func (s *bgAuthService) LoginSellerCentral(ctx context.Context, url string) (string, error) {
	resp, err := s.httpClient.R().
		SetContext(ctx).
		SetDoNotParseResponse(true).
		Get(url)
	if err != nil {
		return "", err
	}
	defer resp.RawResponse.Body.Close()
	return resp.String(), nil
}

// api/seller/auth/loginByCode
func (s *bgAuthService) LoginByCode(ctx context.Context, params BgLoginByCodeRequestParams) (bool, error) {
	if err := params.validate(); err != nil {
		return false, err
	}

	var result = struct {
		normal.Response
		Result struct {
			VerifyAuthToken string `json:"verifyAuthToken"`
		} `json:"result"`
	}{}

	resp, err := s.client.SellerCentralClient.R().
		SetContext(ctx).
		SetResult(&result).
		SetBody(params).
		Post("/api/seller/auth/loginByCode")

	if err != nil {
		return false, err
	}

	err = recheckError(resp, result.Response, err)
	if err != nil {
		return false, err
	}

	return true, nil
}

// 获取登录短信验证码 bg/quiet/api/mms/loginVerifyCode
func (s *bgAuthService) GetLoginVerifyCode(ctx context.Context, params BgGetLoginVerifyCodeRequestParams) (bool, error) {
	if err := params.validate(); err != nil {
		return false, err
	}

	var result = struct {
		normal.ResponseKuajingmaihuo
	}{}

	resp, err := s.httpClient.R().
		SetContext(ctx).
		SetResult(&result).
		SetBody(params).
		Post("/bg/quiet/api/mms/loginVerifyCode")

	if err != nil {
		return false, err
	}

	err = recheckErrorKuajingmaihuo(resp, result.ResponseKuajingmaihuo, err)
	if err != nil {
		return false, err
	}

	return true, nil
}

// // 获取用户信息 api/seller/auth/userInfo
func (s *bgAuthService) GetUserInfo(ctx context.Context) (entity.UserInfo, error) {
	var result = struct {
		normal.Response
		Result entity.UserInfo `json:"result"`
	}{}

	resp, err := s.client.SellerCentralClient.R().
		SetContext(ctx).
		SetBody(map[string]interface{}{}).
		SetResult(&result).
		Post("/api/seller/auth/userInfo")

	if err != nil {
		return entity.UserInfo{}, err
	}

	err = recheckError(resp, result.Response, err)
	if err != nil {
		return entity.UserInfo{}, err
	}

	return result.Result, nil
}

// 获取用户信息 https://seller.kuajingmaihuo.com/bg/quiet/api/mms/userInfo
func (s *bgAuthService) GetMallInfoByKuangjianmaihuo(ctx context.Context) ([]entity.MallInfoByKuangjianmaihuo, error) {
	var result = struct {
		normal.ResponseKuajingmaihuo
		Result struct {
			CompanyList []struct {
				MalInfoList []entity.MallInfoByKuangjianmaihuo `json:"malInfoList"`
			} `json:"companyList"`
		} `json:"result"`
	}{}

	resp, err := s.httpClient.R().
		SetContext(ctx).
		SetResult(&result).
		SetBody(map[string]interface{}{}).
		Post("/bg/quiet/api/mms/userInfo")

	if err != nil {
		return []entity.MallInfoByKuangjianmaihuo{}, err
	}

	err = recheckErrorKuajingmaihuo(resp, result.ResponseKuajingmaihuo, err)
	if err != nil {
		return []entity.MallInfoByKuangjianmaihuo{}, err
	}

	return result.Result.CompanyList[0].MalInfoList, nil
}
