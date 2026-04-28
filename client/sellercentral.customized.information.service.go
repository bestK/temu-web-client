package client

import (
	"context"
	"fmt"

	"github.com/bestK/temu-web-client/entity"
	"github.com/bestK/temu-web-client/normal"
)

// 全托定制信息服务
type customizedInformationService struct {
	service
	client *Client
}

type CustomizedInformationQueryParams struct {
	normal.ParameterWithPage
	SubPurchaseOrderSns []string `json:"subPurchaseOrderSns"` // 备货单号
}

func (s customizedInformationService) Query(ctx context.Context, params CustomizedInformationQueryParams) (items []entity.CustomizedGoods, total, totalPages int, isLastPage bool, err error) {
	params.TidyPager()

	var result = struct {
		normal.Response
		Result struct {
			Total     int                      `json:"total"`
			PageItems []entity.CustomizedGoods `json:"pageItems"`
		} `json:"result"`
	}{}

	if err = s.client.CheckMallId(); err != nil {
		return nil, 0, 0, false, err
	}

	resp, err := s.sellerCentralClient.R().
		SetResult(&result).
		SetContext(ctx).
		SetHeader("mallid", fmt.Sprintf("%d", s.client.MallId)).
		SetBody(params).
		Post("/bg-luna-agent-seller/product/customizeSku/pageQuery")

	if err = recheckError(resp, result.Response, err); err != nil {
		s.client.Logger.Errorf("查询备货单定制信息失败: %v %+v", err, string(resp.Body()))
		return nil, 0, 0, false, err
	}

	items = result.Result.PageItems
	total, totalPages, isLastPage = parseResponseTotal(params.Page, params.PageSize, result.Result.Total)
	if !isLastPage {
		params.Page++
		nextItems, nextTotal, _, _, err := s.Query(ctx, params) // 递归获取剩余页
		if err != nil {
			return items, total, totalPages, isLastPage, err
		}
		items = append(items, nextItems...)
		total += nextTotal
	}
	return items, total, totalPages, isLastPage, nil
}
