package v1

import "github.com/gogf/gf/v2/frame/g"

// 后台管理员接口。没有公开注册：第一个超级管理员用命令行创建，
// 之后只有超级管理员能通过 AdminCreate 添加新管理员。

type AdminLoginReq struct {
	g.Meta   `path:"/admin/login" method:"post" tags:"管理员" summary:"管理员登录"`
	Name     string `json:"name"     v:"required" dc:"管理员名"`
	Password string `json:"password" v:"required" dc:"密码"`
}

type AdminLoginRes struct {
	TokenType   string `json:"tokenType"   dc:"固定为 Bearer"`
	AccessToken string `json:"accessToken" dc:"管理员令牌，放进 Authorization 头"`
	ExpiresIn   uint32 `json:"expiresIn"   dc:"有效期（秒）"`
	AdminId     uint64 `json:"adminId"     dc:"管理员 ID"`
	Name        string `json:"name"        dc:"管理员名"`
	IsSuper     bool   `json:"isSuper"     dc:"是否超级管理员"`
}

type AdminCreateReq struct {
	g.Meta   `path:"/admin" method:"post" tags:"管理员" summary:"创建管理员（仅超级管理员）"`
	Name     string `json:"name"     v:"required|length:1,64"   dc:"管理员名"`
	Password string `json:"password" v:"required|length:12,128" dc:"密码，至少 12 位"`
	IsSuper  bool   `json:"isSuper"  dc:"是否超级管理员"`
}

type AdminCreateRes struct {
	AdminId uint64 `json:"adminId" dc:"新管理员 ID"`
}
