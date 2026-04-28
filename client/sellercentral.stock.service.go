package client

import (
	"context"
	"fmt"

	"github.com/bestK/temu-web-client/entity"
	"github.com/bestK/temu-web-client/normal"
	validation "github.com/go-ozzo/ozzo-validation/v4"
	"gopkg.in/guregu/null.v4"
)

type stockService struct {
	service
	client *Client
}

type UpdateMmsBtgProductSalesStockRequestParams struct {
	ProductId          int              `json:"productId"`
	SkuStockChangeList []SkuStockChange `json:"skuStockChangeList"`
	SkuTypeChangeList  []SkuTypeChange  `json:"skuTypeChangeList,omitempty"`
	IsCheckVersion     bool             `json:"isCheckVersion"`
}

type SkuStockChange struct {
	ProductSkuId          int    `json:"productSkuId"`
	StockDiff             int    `json:"stockDiff"`
	CurrentStockAvailable int    `json:"currentStockAvailable"`
	CurrentShippingMode   int    `json:"currentShippingMode"`
	WarehouseId           string `json:"warehouseId"`
}

type SkuTypeChange struct {
	ProductSkuId   int `json:"productSkuId"`
	ProductSkuType int `json:"productSkuType"`
}

type QueryBtgProductStockInfoRequestParams struct {
	ProductId        null.Int `json:"productId,omitempty"`
	ProductSkuIdList []int    `json:"productSkuIdList"`
}

func (m UpdateMmsBtgProductSalesStockRequestParams) validate() error {
	return validation.ValidateStruct(&m,
		validation.Field(&m.ProductId, validation.Required.Error("商品ID不能为空")),
		validation.Field(&m.SkuStockChangeList, validation.Required.Error("SKU库存变更列表不能为空")),
	)
}

// 查询SKU库存信息 /darwin-mms/api/kiana/ghost/btg/sales/stock/queryBtgProductStockInfo
func (s stockService) QueryBtgProductStockInfo(ctx context.Context, params QueryBtgProductStockInfoRequestParams) ([]entity.ProductStock, error) {
	var result = struct {
		normal.Response `json:",inline"`
		Result          struct {
			ProductStockList []entity.ProductStock `json:"productStockList"`
		} `json:"result,omitempty"`
	}{}

	if err := s.client.CheckMallId(); err != nil {
		return nil, err
	}

	resp, err := s.sellerCentralClient.R().
		SetHeader("mallid", fmt.Sprintf("%d", s.client.MallId)).
		SetResult(&result).
		SetContext(ctx).
		SetBody(params).
		Post("/darwin-mms/api/kiana/ghost/btg/sales/stock/queryBtgProductStockInfo")
	if err = recheckError(resp, result.Response, err); err != nil {
		s.logger.Errorf("查询SKU库存信息失败: %v %+v", err, string(resp.Body()))
		return nil, err
	}

	return result.Result.ProductStockList, nil
}

// 修改库存 /darwin-mms/api/kiana/ghost/btg/sales/stock/updateMmsBtgProductSalesStock
func (s stockService) UpdateMmsBtgProductSalesStock(ctx context.Context, params UpdateMmsBtgProductSalesStockRequestParams) (bool, error) {
	if err := params.validate(); err != nil {
		return false, err
	}

	if err := s.client.CheckMallId(); err != nil {
		return false, err
	}

	var result = struct {
		normal.Response `json:",inline"`
		Result          struct {
			SkuMaxQuantityDTOList []interface{} `json:"skuMaxQuantityDTOList"`
			IsSuccess             bool          `json:"isSuccess"`
		} `json:"result,omitempty"`
	}{}

	resp, err := s.sellerCentralClient.R().
		SetHeader("mallid", fmt.Sprintf("%d", s.client.MallId)).
		SetResult(&result).
		SetContext(ctx).
		SetBody(params).
		Post("/darwin-mms/api/kiana/ghost/btg/sales/stock/updateMmsBtgProductSalesStock")
	if err = recheckError(resp, result.Response, err); err != nil {
		s.logger.Errorf("更新库存失败: %v %+v", err, string(resp.Body()))
		return false, err
	}

	return result.Result.IsSuccess, nil
}
