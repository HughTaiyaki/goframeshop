package v1

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

type UserRegisterReq struct {
	g.Meta   `path:"/user/register" method:"post" tags:"用户" summary:"注册"`
	Username string `json:"username" v:"required|length:1,200"  dc:"用户名"`
	Password string `json:"password" v:"required|length:8,128"  dc:"密码"`
	Avatar   string `json:"avatar"                              dc:"头像地址"`
	Sex      uint32 `json:"sex"                                 dc:"0 未知 1 男 2 女"`
	Sign     string `json:"sign"                                dc:"个性签名"`
}

type UserRegisterRes struct {
	UserId uint64 `json:"userId" dc:"用户 ID"`
}

type UserLoginReq struct {
	g.Meta   `path:"/user/login" method:"post" tags:"用户" summary:"登录"`
	Username string `json:"username" v:"required" dc:"用户名"`
	Password string `json:"password" v:"required" dc:"密码"`
}

// UserLoginRes carries the access token. Send it back on later requests as
// "Authorization: Bearer <accessToken>".
type UserLoginRes struct {
	TokenType   string      `json:"tokenType"   dc:"固定为 Bearer"`
	AccessToken string      `json:"accessToken" dc:"访问令牌，放进 Authorization 头"`
	ExpiresIn   uint32      `json:"expiresIn"   dc:"令牌有效期（秒）"`
	UserId      uint64      `json:"userId"      dc:"用户 ID"`
	Username    string      `json:"username"    dc:"用户名"`
	Avatar      string      `json:"avatar"      dc:"头像地址"`
	Sex         uint32      `json:"sex"         dc:"性别"`
	Sign        string      `json:"sign"        dc:"个性签名"`
	Status      uint32      `json:"status"      dc:"1 正常 2 封禁"`
	CreateAt    *gtime.Time `json:"createdAt"   dc:"注册时间"`
}

type UserProfileReq struct {
	g.Meta `path:"/user/profile" method:"get" tags:"用户" summary:"查询当前用户资料"`
}

type UserProfileRes struct {
	UserId   uint64      `json:"userId"    dc:"用户 ID"`
	Username string      `json:"username"  dc:"用户名"`
	Avatar   string      `json:"avatar"    dc:"头像地址"`
	Sex      uint32      `json:"sex"       dc:"性别"`
	Sign     string      `json:"sign"      dc:"个性签名"`
	Status   uint32      `json:"status"    dc:"1 正常 2 封禁"`
	CreateAt *gtime.Time `json:"createdAt" dc:"注册时间"`
}

// 下面两个接口都需要登录，改的是令牌里那个用户，请求里不能指定用户 ID。

type UserPasswordUpdateReq struct {
	g.Meta          `path:"/user/password" method:"put" tags:"用户" summary:"修改密码（需要当前密码）"`
	CurrentPassword string `json:"currentPassword" v:"required"              dc:"当前密码"`
	NewPassword     string `json:"newPassword"     v:"required|length:8,128" dc:"新密码"`
}

// UserPasswordUpdateRes 为空。已经签发的令牌在过期前仍然有效（还没有吊销机制）。
type UserPasswordUpdateRes struct{}

type UserProfileUpdateReq struct {
	g.Meta `path:"/user/profile" method:"put" tags:"用户" summary:"修改资料（整体替换头像、性别、签名）"`
	Avatar string `json:"avatar" v:"length:0,512" dc:"头像地址，空或 http(s) 绝对地址；空表示清除"`
	Sex    uint32 `json:"sex"    v:"in:0,1,2"     dc:"0 未知 1 男 2 女"`
	Sign   string `json:"sign"   v:"length:0,255" dc:"个性签名，空表示清除"`
}

type UserProfileUpdateRes struct {
	*UserProfileRes
}
