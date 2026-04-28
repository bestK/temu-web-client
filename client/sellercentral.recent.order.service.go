// 最近订单服务
package client

import (
	"context"
	"fmt"

	"github.com/bestK/temu-web-client/entity"
	"github.com/bestK/temu-web-client/normal"
	"gopkg.in/guregu/null.v4"
)

type recentOrderService struct {
	service
	client *Client
}

type RecentOrderQueryParams struct {
	normal.ParameterWithPager
	FulfillmentMode     null.Int    `json:"fulfillmentMode"`
	QueryType           null.Int    `json:"queryType"`
	SortType            null.Int    `json:"sortType"`
	ParentOrderSnList   []string    `json:"parentOrderSnList"`
	TimeZone            null.String `json:"timeZone"`
	ParentAfterSalesTag null.Int    `json:"parentAfterSalesTag"`
	NeedBuySignService  null.Int    `json:"needBuySignService"`
	SellerNoteLabelList []int       `json:"sellerNoteLabelList"`
}

// Query 查询订单列表
func (s recentOrderService) Query(ctx context.Context, params RecentOrderQueryParams) (items []entity.RecentOrder, total, totalPages int, isLastPage bool, err error) {
	params.TidyPager()

	var result = struct {
		normal.Response
		Result struct {
			TotalItemNum int                  `json:"totalItemNum"`
			PageItems    []entity.RecentOrder `json:"pageItems"`
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
		Post("/kirogi/bg/mms/recentOrderList")

	if err = recheckError(resp, result.Response, err); err != nil {
		s.client.Logger.Errorf("查询最近订单列表失败: %v %+v", err, string(resp.Body()))
		return nil, 0, 0, false, err
	}

	items = result.Result.PageItems
	total, totalPages, isLastPage = parseResponseTotal(params.Page, params.PageSize, result.Result.TotalItemNum)
	if !isLastPage {
		params.Page++
		nextItems, nextTotal, _, _, err := s.Query(ctx, params) // 递归获取
		if err != nil {
			return items, total, totalPages, isLastPage, err
		}
		items = append(items, nextItems...)
		total += nextTotal
	}
	return items, total, totalPages, isLastPage, err
}
