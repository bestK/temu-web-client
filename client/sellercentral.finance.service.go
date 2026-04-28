package client

import (
	"context"
	"fmt"

	"github.com/bestK/temu-web-client/entity"
	"github.com/bestK/temu-web-client/normal"
)

// 财务服务
type financeService struct {
	service
	client *Client
}

// AccountFunds 财务账户资金
func (s financeService) AccountFunds(ctx context.Context) (d entity.FinanceAccountFunds, err error) {
	var result = struct {
		normal.Response
		Result entity.FinanceAccountFunds `json:"result"`
	}{}

	if err = s.client.CheckMallId(); err != nil {
		return d, err
	}

	resp, err := s.sellerCentralClient.R().
		SetResult(&result).
		SetContext(ctx).
		SetHeader("mallid", fmt.Sprintf("%d", s.client.MallId)).
		Post("/api/merchant/payment/account/amount/info")

	if err = recheckError(resp, result.Response, err); err != nil {
		s.client.Logger.Errorf("查询财务货款账户信息失败: %v %+v", err, string(resp.Body()))
		return
	}

	return result.Result, nil
}
