package client

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/bestK/temu-web-client/config"
	"github.com/bestK/temu-web-client/entity"
	"github.com/bestK/temu-web-client/normal"
	"github.com/bestK/temu-web-client/utils"
	"github.com/goccy/go-json"
	"github.com/stretchr/testify/assert"
	"gopkg.in/guregu/null.v4"
)

var temuClient *Client

var ctx = context.Background()

func TestMain(m *testing.M) {
	b, err := os.ReadFile("../config/config_test.json")
	if err != nil {
		panic(fmt.Sprintf("Read config error: %s", err.Error()))
	}
	var cfg config.TemuBrowserConfig
	err = json.Unmarshal(b, &cfg)
	if err != nil {
		panic(fmt.Sprintf("Parse config file error: %s", err.Error()))
	}

	temuClient = NewClient(cfg)
	m.Run()
}

func TestGetAntiContent(t *testing.T) {
	antiContent, err := utils.GetAntiContent()
	if err != nil {
		t.Errorf("获取 Anti-Content 失败: %v", err)
	}

	if antiContent == "" {
		t.Error("获取的 Anti-Content 为空")
	}
	t.Logf("获取的 Anti-Content: %s", antiContent)
}

// 测试登录
func TestLogin(t *testing.T) {
	loginName := ""
	password := ""
	mallId := 0
	verifyCode := null.NewString("", false)

	publicKey, _, err := temuClient.Services.BgAuthService.GetPublicKey()
	if err != nil {
		t.Errorf("获取公钥失败: %v", err)
	}

	encryptPassword, err := utils.EncryptPassword(password, publicKey)
	if err != nil {
		t.Errorf("加密密码失败: %v", err)
	}

	bgLoginRequestParams := BgLoginRequestParams{
		LoginName:       loginName,
		EncryptPassword: encryptPassword,
		KeyVersion:      "1",
		VerifyCode:      verifyCode.Ptr(),
	}

	accountId, cookies, err := temuClient.Services.BgAuthService.Login(ctx, bgLoginRequestParams)
	if err != nil {
		if errors.Is(err, normal.ErrNeedSMSCode) {
			t.Logf("需要短信验证码: %v", err)
			// success, err := temuClient.Services.BgAuthService.GetLoginVerifyCode(ctx, BgGetLoginVerifyCodeRequestParams{
			// 	Mobile: loginName,
			// })
			// if err != nil {
			// 	t.Errorf("获取短信验证码失败: %v", err)
			// }
			// if !success {
			// 	t.Error("获取短信验证码失败")
			// }

			accountId, cookies, err = temuClient.Services.BgAuthService.Login(ctx, bgLoginRequestParams)
			if err != nil {
				t.Errorf("登录失败: %v", err)
				panic(err)
			}
			t.Logf("登录成功，返回的 AccountId: %d", accountId)
			t.Logf("登录成功，返回的 Cookies: %v", cookies)
		} else {
			t.Errorf("登录失败: %v", err)
		}
	}

	if accountId == 0 {
		t.Error("登录成功，但返回的 AccountId 为空")
	}
	t.Logf("登录成功，返回的 AccountId: %d", accountId)

	// 获取验证码
	code, err := temuClient.Services.BgAuthService.ObtainCode(ctx, BgObtainCodeRequestParams{
		RedirectUrl: "https://agentseller.temu.com/main/authentication",
	})
	if err != nil {
		t.Errorf("获取验证码失败: %v", err)
	}
	t.Logf("获取验证码成功，返回的验证码: %s", code)

	loginByCodeParams := BgLoginByCodeRequestParams{
		Code:         code,
		Confirm:      false,
		TargetMallId: mallId,
	}
	temuClient.SetMallId(mallId)
	success, _, err := temuClient.Services.BgAuthService.LoginSellerCentralByCode(ctx, loginByCodeParams)
	if err != nil {
		t.Errorf("登录 Seller Central 失败: %v", err)
		panic(err)
	}
	t.Logf("登录 Seller Central 成功: %v", success)

	userInfo, err := temuClient.Services.BgAuthService.GetSellerCentralUserInfo(ctx)
	if err != nil {
		t.Errorf("获取用户信息失败: %v", err)
		panic(err)
	}
	t.Logf("获取用户信息成功: %+v", userInfo)
}

// 测试通过 Cookie 直接登录并获取用户信息
// 依赖 config.SellerCentralCookie 已配置；可选 region 由 TEMU_REGION 指定
func TestLoginByCookie(t *testing.T) {

	if err := temuClient.Services.BgAuthService.LoginByCookie(ctx, ""); err != nil {
		t.Fatalf("通过 Cookie 登录失败 (region=%q): %v", "", err)
	}

	userInfo, err := temuClient.Services.BgAuthService.GetSellerCentralUserInfo(ctx)
	if err != nil {
		t.Fatalf("获取用户信息失败: %v", err)
	}
	t.Logf("获取用户信息成功: %+v", userInfo)
	assert.NotEmpty(t, userInfo)
}

func TestRecentOrder(t *testing.T) {
	TestLogin(t)
	// 查询订单列表
	params := RecentOrderQueryParams{
		QueryType:           null.NewInt(entity.RecentOrderStatusUnshipped, true),
		FulfillmentMode:     null.NewInt(0, true),
		SortType:            null.NewInt(1, true),
		TimeZone:            null.NewString("UTC+8", true),
		ParentAfterSalesTag: null.NewInt(0, true),
		NeedBuySignService:  null.NewInt(0, true),
		SellerNoteLabelList: []int{},
		ParentOrderSnList:   []string{},
	}
	items, total, _, _, err := temuClient.Services.RecentOrderService.Query(ctx, params)
	if err != nil {
		t.Errorf("查询订单列表失败: %v", err)
	}
	t.Logf("查询订单列表成功，返回的订单数量: %d", total)
	for _, item := range items {
		for _, order := range item.OrderList {
			t.Logf("子订单信息: %+v %+v %+v", order.OrderSn, order.OrderStatus, order.ProductInfoList)
		}
	}
	productId := int64(523914601)
	stockList, err := temuClient.Services.StockService.QueryBtgProductStockInfo(ctx, QueryBtgProductStockInfoRequestParams{
		ProductId:        null.NewInt(productId, true),
		ProductSkuIdList: []int{7634297783},
	})
	if err != nil {
		t.Errorf("查询SKU库存信息失败: %v", err)
		panic(err)
	}
	t.Logf("查询SKU库存信息成功，返回的库存列表: %v", stockList)
	for _, stock := range stockList {
		jsonString, _ := json.Marshal(stock)
		t.Logf("库存信息: %+v", string(jsonString))
	}
}

func TestCustomizedInformation(t *testing.T) {
	TestLogin(t)
	gotItems, gotTotal, gotTotalPages, gotIsLastPage, err := temuClient.Services.CustomizedInformationService.Query(ctx, CustomizedInformationQueryParams{SubPurchaseOrderSns: []string{"WB2507101860720"}})
	assert.Equal(t, err, nil)
	assert.Equal(t, 2, len(gotItems))
	assert.Equal(t, 2, gotTotal)
	assert.Equal(t, 1, gotTotalPages)
	assert.Equal(t, true, gotIsLastPage)
}

// TestUpdateStock 测试修改库存接口。
//
// 由于会真实修改远端库存，默认跳过；通过环境变量启用：
//
//	TEMU_TEST_UPDATE_STOCK=1   开启此用例
//	TEMU_PRODUCT_ID            商品 ID
//	TEMU_PRODUCT_SKU_ID        商品 SKU ID
//	TEMU_STOCK_DIFF            库存增量（必填，可正可负；服务端不接受 0）
//
// 流程：先 Query 当前库存 → 用 CurrentStockAvailable + StockDiff 调用 Update。
func TestUpdateStock(t *testing.T) {
	if os.Getenv("TEMU_TEST_UPDATE_STOCK") == "" {
		t.Skip("未设置 TEMU_TEST_UPDATE_STOCK，跳过修改库存用例")
	}

	// TestLoginByCookie(t)
	// if t.Failed() {
	// 	t.FailNow()
	// }

	var (
		mallId       int
		productId    int64
		productSkuId int
		stockDiff    int
	)
	if v := os.Getenv("TEMU_MALL_ID"); v != "" {
		fmt.Sscanf(v, "%d", &mallId)
	}
	if v := os.Getenv("TEMU_PRODUCT_ID"); v != "" {
		fmt.Sscanf(v, "%d", &productId)
	}
	if v := os.Getenv("TEMU_PRODUCT_SKU_ID"); v != "" {
		fmt.Sscanf(v, "%d", &productSkuId)
	}
	if v := os.Getenv("TEMU_STOCK_DIFF"); v != "" {
		fmt.Sscanf(v, "%d", &stockDiff)
	}
	if mallId == 0 || productId == 0 || productSkuId == 0 {
		t.Skip("未提供 TEMU_MALL_ID / TEMU_PRODUCT_ID / TEMU_PRODUCT_SKU_ID，跳过")
	}

	temuClient.SetMallId(mallId)

	// 1. 查询当前库存
	stockList, err := temuClient.Services.StockService.QueryBtgProductStockInfo(ctx, QueryBtgProductStockInfoRequestParams{
		ProductId:        null.NewInt(productId, true),
		ProductSkuIdList: []int{productSkuId},
	})
	if err != nil {
		t.Fatalf("查询当前库存失败: %v", err)
	}
	if len(stockList) == 0 {
		t.Fatalf("未查询到 productSkuId=%d 的库存", productSkuId)
	}
	stock := stockList[0]
	if len(stock.WarehouseStockList) == 0 {
		t.Fatalf("productSkuId=%d 没有任何仓库库存", productSkuId)
	}
	wh := stock.WarehouseStockList[0]
	t.Logf("当前库存: skuId=%d warehouse=%s available=%d shippingMode=%d",
		stock.ProductSkuId, wh.WarehouseInfo.WarehouseId, wh.StockAvailable, stock.ShippingMode)

	// 2. 提交修改（diff=0 即无副作用的连通性测试）
	ok, err := temuClient.Services.StockService.UpdateMmsBtgProductSalesStock(ctx, UpdateMmsBtgProductSalesStockRequestParams{
		ProductId: int(productId),
		SkuStockChangeList: []SkuStockChange{
			{
				ProductSkuId:          productSkuId,
				StockDiff:             stockDiff,
				CurrentStockAvailable: wh.StockAvailable,
				CurrentShippingMode:   stock.ShippingMode,
				WarehouseId:           wh.WarehouseInfo.WarehouseId,
			},
		},
		IsCheckVersion: true,
	})
	if err != nil {
		t.Fatalf("修改库存失败: %v", err)
	}
	t.Logf("修改库存返回: success=%v (diff=%d)", ok, stockDiff)
	assert.True(t, ok)
}

func TestFinanceAccountFunds(t *testing.T) {
	TestLogin(t)
	accountFunds, err := temuClient.Services.FinanceService.AccountFunds(ctx)
	assert.Equal(t, err, nil)
	assert.Equal(t, "13950.20", accountFunds.TotalAmount)
}
