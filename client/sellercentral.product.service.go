package client

import (
	"context"
	"fmt"

	"github.com/bestK/temu-web-client/entity"
	"github.com/bestK/temu-web-client/normal"
	"gopkg.in/guregu/null.v4"
)

type productService struct {
	service
	client *Client
}

type ProductQueryParams struct {
	normal.ParameterWithPage
	SkcTopStatus         null.Int             `json:"skcTopStatus"`
	SkuExtCodes          []string             `json:"skuExtCodes"`          // 货号
	ProductIds           []int                `json:"productIds"`           // 商品ID
	ProductSkcIds        []int                `json:"productSkcIds"`        // Skc
	ProductName          string               `json:"productName"`          // 商品名称
	StockQuantitySection StockQuantitySection `json:"stockQuantitySection"` // 库存数量区间
	SkcSiteStatus        null.Int             `json:"skcSiteStatus"`        // 在售状态 1:在售 0:下架
}

type StockQuantitySection struct {
	LeftValue  int `json:"leftValue"`
	RightValue int `json:"rightValue"`
}

func (s productService) Query(ctx context.Context, params ProductQueryParams) (items []entity.Product, total, totalPages int, isLastPage bool, err error) {
	params.TidyPager()

	var result = struct {
		normal.Response
		Result struct {
			Total     int              `json:"total"`
			PageItems []entity.Product `json:"pageItems"`
		} `json:"result"`
	}{}

	if err := s.client.CheckMallId(); err != nil {
		return nil, 0, 0, false, err
	}

	resp, err := s.sellerCentralClient.R().
		SetResult(&result).
		SetContext(ctx).
		SetHeader("mallid", fmt.Sprintf("%d", s.client.MallId)).
		SetBody(params).
		Post("/visage-agent-seller/product/skc/pageQuery")

	if err = recheckError(resp, result.Response, err); err != nil {
		s.client.Logger.Errorf("查询商品列表失败: %v %+v", err, string(resp.Body()))
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
