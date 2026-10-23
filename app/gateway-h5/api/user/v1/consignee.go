package v1

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// The gateway speaks HTTP/JSON to the outside world and gRPC to the user
// service. These structs are the HTTP side: they are what the browser sends and
// receives, and they are intentionally not the same structs as the gRPC ones.

type ConsigneeCreateReq struct {
	g.Meta    `path:"/consignee" method:"post" tags:"收货地址" summary:"创建收货地址"`
	IsDefault bool   `json:"isDefault" dc:"是否设为默认地址"`
	Name      string `json:"name"      v:"required|length:1,64"  dc:"收货人姓名"`
	Phone     string `json:"phone"     v:"required|length:1,32"  dc:"联系电话"`
	Province  string `json:"province"  v:"required|length:1,64"  dc:"省份"`
	City      string `json:"city"      v:"required|length:1,64"  dc:"城市"`
	Town      string `json:"town"      v:"required|length:1,64"  dc:"区县"`
	Street    string `json:"street"    v:"required|length:1,128" dc:"街道"`
	Detail    string `json:"detail"    v:"required|length:1,255" dc:"详细地址"`
}

type ConsigneeCreateRes struct {
	ConsigneeId uint64 `json:"consigneeId" dc:"新建地址 ID"`
}

type ConsigneeListReq struct {
	g.Meta `path:"/consignee" method:"get" tags:"收货地址" summary:"收货地址列表"`
	Page   uint32 `json:"page" v:"min:1"     d:"1"  dc:"页码，从 1 开始"`
	Size   uint32 `json:"size" v:"max:100"   d:"10" dc:"每页条数"`
}

type ConsigneeListRes struct {
	Items []*ConsigneeItem `json:"items" dc:"地址列表"`
	Page  uint32           `json:"page"  dc:"当前页码"`
	Size  uint32           `json:"size"  dc:"每页条数"`
	Total uint64           `json:"total" dc:"总数"`
}

type ConsigneeUpdateReq struct {
	g.Meta      `path:"/consignee" method:"put" tags:"收货地址" summary:"修改收货地址"`
	ConsigneeId uint64 `json:"consigneeId" v:"required|min:1"    dc:"地址 ID"`
	IsDefault   bool   `json:"isDefault"   dc:"是否设为默认地址"`
	Name        string `json:"name"        v:"required|length:1,64"  dc:"收货人姓名"`
	Phone       string `json:"phone"       v:"required|length:1,32"  dc:"联系电话"`
	Province    string `json:"province"    v:"required|length:1,64"  dc:"省份"`
	City        string `json:"city"        v:"required|length:1,64"  dc:"城市"`
	Town        string `json:"town"        v:"required|length:1,64"  dc:"区县"`
	Street      string `json:"street"      v:"required|length:1,128" dc:"街道"`
	Detail      string `json:"detail"      v:"required|length:1,255" dc:"详细地址"`
}

type ConsigneeUpdateRes struct {
	ConsigneeId uint64 `json:"consigneeId" dc:"地址 ID"`
}

type ConsigneeDeleteReq struct {
	g.Meta      `path:"/consignee" method:"delete" tags:"收货地址" summary:"删除收货地址"`
	ConsigneeId uint64 `json:"consigneeId" v:"required|min:1" dc:"地址 ID"`
}

type ConsigneeDeleteRes struct{}

// ConsigneeItem is the JSON shape of one address. No deleted_at here: soft
// deletion is internal to the user service.
type ConsigneeItem struct {
	ConsigneeId uint64      `json:"consigneeId" dc:"地址 ID"`
	IsDefault   bool        `json:"isDefault"   dc:"是否默认地址"`
	Name        string      `json:"name"        dc:"收货人姓名"`
	Phone       string      `json:"phone"       dc:"联系电话"`
	Province    string      `json:"province"    dc:"省份"`
	City        string      `json:"city"        dc:"城市"`
	Town        string      `json:"town"        dc:"区县"`
	Street      string      `json:"street"      dc:"街道"`
	Detail      string      `json:"detail"      dc:"详细地址"`
	CreatedAt   *gtime.Time `json:"createdAt"   dc:"创建时间"`
	UpdatedAt   *gtime.Time `json:"updatedAt"   dc:"更新时间"`
}
