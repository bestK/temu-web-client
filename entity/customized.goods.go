package entity

import (
	"fmt"

	gci "github.com/echo-ok/goods-customization-information"
	"gopkg.in/guregu/null.v4"
)

type CustomizedInformationSurface struct {
	BaseImage struct {
		ImageUrl string `json:"imageUrl"`
	} `json:"baseImage"`
	MaskImage struct {
		ImageUrl string `json:"imageUrl"`
	} `json:"maskImage"`
	Regions []struct {
		Elements []struct {
			LengthLimit int         `json:"lengthLimit"`
			Require     bool        `json:"require"`
			MaxSize     interface{} `json:"maxSize"`
			Type        int         `json:"type"`
			TextAlign   int         `json:"textAlign"`
		} `json:"elements"`
		Position struct {
			X int `json:"x"`
			Y int `json:"y"`
		} `json:"position"`
		Dimension struct {
			Width  int `json:"width"`
			Height int `json:"height"`
		} `json:"dimension"`
	} `json:"regions"`
}

type CustomizedInformationPreviewItem struct {
	PreviewType     int         `json:"previewType"` // 1: overall preview image, 3: image, 4: text
	ImageUrl        null.String `json:"imageUrl"`
	ImageUrlDisplay null.String `json:"imageUrlDisplay"`
	CustomizedText  null.String `json:"customizedText"`
	RegionId        null.String `json:"regionId"`
}
type RawCustomizedInformation struct {
	TemplateId                int64                              `json:"templateId"`
	TemplateType              int                                `json:"templateType"`
	CustomizationTmplSurfaces []CustomizedInformationSurface     `json:"customizationTmplSurfaces"`
	CustomizedPreviewItems    []CustomizedInformationPreviewItem `json:"customizedPreviewItems"`
}

// Parse 定制信息解析
func (rci RawCustomizedInformation) Parse() (ci CustomizedInformation, err error) {
	if len(rci.CustomizedPreviewItems) == 0 {
		return ci, fmt.Errorf("no preview items")
	}
	ci.Texts = make([]LabeledValue, 0)
	ci.Images = make([]string, 0)
	for _, previewItem := range rci.CustomizedPreviewItems {
		switch previewItem.PreviewType {
		case 1:
			ci.PreviewImage = previewItem.ImageUrlDisplay
		case 3:
			if !previewItem.ImageUrlDisplay.Valid {
				return ci, fmt.Errorf("no image url")
			}
			ci.Images = append(ci.Images, previewItem.ImageUrlDisplay.String)
		case 4:
			if !previewItem.RegionId.Valid || !previewItem.CustomizedText.Valid {
				return ci, fmt.Errorf("no region id or customized text")
			}
			ci.Texts = append(ci.Texts, LabeledValue{
				Label: previewItem.RegionId.String,
				Value: previewItem.CustomizedText.String,
			})
		default:
			return ci, fmt.Errorf("unknown preview type: %d", previewItem.PreviewType)
		}
	}
	return
}

// NormalizedParse 定制信息解析
func (rci RawCustomizedInformation) NormalizedParse() (g gci.GoodsCustomizedInformation, err error) {
	if len(rci.CustomizedPreviewItems) == 0 {
		return g, fmt.Errorf("no preview items")
	}

	g = gci.NewGoodsCustomizedInformation("")
	g.SetRawData(rci)
	surface := gci.NewSurface() // Temu 都只支持一个面的定制
	kvRegion := make(map[string]gci.Region)
	for _, previewItem := range rci.CustomizedPreviewItems {
		if previewItem.PreviewType == 1 {
			// Preview image
			if previewItem.ImageUrlDisplay.Valid {
				image, e := gci.NewImage(previewItem.ImageUrlDisplay.String, false)
				if e != nil {
					image.SetError(e)
				}
				surface.SetPreviewImage(image)
			}
			continue
		}
		var region gci.Region
		regionId := previewItem.RegionId.ValueOrZero()
		if v, ok := kvRegion[regionId]; ok {
			region = v
		} else {
			region = gci.NewRegion(regionId)
		}
		switch previewItem.PreviewType {
		case 3: // Upload image
			if !previewItem.ImageUrlDisplay.Valid {
				return g, fmt.Errorf("%s ImageUrlDisplay value is empty", regionId)
			}
			image, e := gci.NewImage(previewItem.ImageUrlDisplay.String, false)
			if e != nil {
				image.SetError(e)
			}
			region.AddImage(image)
		case 4: // Customization text
			if !previewItem.CustomizedText.Valid {
				return g, fmt.Errorf("%s customization text is empty", regionId)
			}
			text, e := gci.NewText("", previewItem.CustomizedText.String)
			if e != nil {
				region.SetError(e)
			}
			region.AddText(text)
		default:
			return g, fmt.Errorf("unknown preview type: %d", previewItem.PreviewType)
		}
		kvRegion[regionId] = region
	}
	for _, region := range kvRegion {
		surface.AddRegion(region)
	}
	g.AddSurface(surface)
	return
}

// CustomizedGoods 定制商品
type CustomizedGoods struct {
	OrderSn              string `json:"orderSn"`
	PersonalProductSkuId int64  `json:"personalProductSkuId"`
	ProductId            int64  `json:"productId"`
	ProductSkcId         int64  `json:"productSkcId"`
	ProductSkuId         int64  `json:"productSkuId"`
	ProductName          string `json:"productName"`
	DisplayImage         string `json:"displayImage"`
	SkuThumbUrl          string `json:"skuThumbUrl"`
	ExtCode              string `json:"extCode"`
	ProductSkuSpecList   []struct {
		ParentId   interface{} `json:"parentId"`
		ParentName interface{} `json:"parentName"`
		SpecId     int         `json:"specId"`
		SpecName   string      `json:"specName"`
	} `json:"productSkuSpecList"`
	ProductSkuPersonalInfoList interface{}              `json:"productSkuPersonalInfoList"`
	PersonalizationTmp         interface{}              `json:"personalizationTmp"`
	ProductSkuCustomization    RawCustomizedInformation `json:"productSkuCustomization"` // 定制信息
	SubPurchaseOrderSnList     []string                 `json:"subPurchaseOrderSnList"`
	SubPurchaseOrderInfoVOS    []struct {
		SubPurchaseOrderSn string `json:"subPurchaseOrderSn"`
		PurchaseQuantity   int    `json:"purchaseQuantity"`
	} `json:"subPurchaseOrderInfoVOS"`
	ProductTechnologyVO struct {
		ProcessType       int   `json:"processType"`
		FirstProcessType  int   `json:"firstProcessType"`
		SecondProcessType []int `json:"secondProcessType"`
	} `json:"productTechnologyVO"`
	UsOrder                    bool          `json:"usOrder"`
	CustomizedSkuAppealRecords []interface{} `json:"customizedSkuAppealRecords"`
}

type LabeledValue struct {
	Label string `json:"label"`
	Value string `json:"value"`
}
type CustomizedInformation struct {
	PreviewImage null.String    `json:"preview_image"` // 预览图
	Texts        []LabeledValue `json:"texts"`         // 定制文本
	Images       []string       `json:"images"`        // 定制图片
}
