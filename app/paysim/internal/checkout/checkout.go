// Package checkout is the payer's side of the simulator: the page a pay_url
// opens, and the button that pays. A real provider would authenticate the
// payer here; the simulator has no payers, so whoever opens the link pays.
//
//	GET  /checkout/{paymentId}               the payment and a pay button
//	POST /checkout/{paymentId}/pay           pay; JSON answer
//	POST /checkout/{paymentId}/pay?notify=0  pay without notifying the merchant
//	                                         (a lost notification, for drills)
package checkout

import (
	"errors"
	"fmt"
	"html/template"
	"net/http"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"

	"github.com/hughtaiyaki/goframeshop/app/paysim/internal/service"
)

func Register(s *ghttp.Server, provider service.IProvider) {
	h := &handler{provider: provider}
	s.BindHandler("GET:/checkout/{paymentId}", h.page)
	s.BindHandler("POST:/checkout/{paymentId}/pay", h.pay)
}

type handler struct {
	provider service.IProvider
}

var pageTemplate = template.Must(template.New("checkout").Parse(`<!doctype html>
<html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>支付模拟器</title>
<style>body{font-family:system-ui,sans-serif;max-width:28rem;margin:2rem auto;padding:0 1rem;color:#222}
dl{display:grid;grid-template-columns:auto 1fr;gap:.4rem 1rem}dt{color:#666}
button{font-size:1rem;padding:.6rem 1.2rem;margin:.3rem .3rem 0 0;cursor:pointer}#result{margin-top:1rem;white-space:pre-wrap}</style>
</head><body>
<h1>支付模拟器</h1>
<p>这是本地演示用的假收银台，不涉及真实资金。</p>
<dl>
<dt>支付单</dt><dd>{{.ID}}</dd>
<dt>订单号</dt><dd>{{.OrderNo}}</dd>
<dt>金额</dt><dd>{{.Yuan}} {{.Currency}}</dd>
<dt>状态</dt><dd>{{.State}}</dd>
{{if .ExpiresAt}}<dt>截止</dt><dd>{{.ExpiresAt}}</dd>{{end}}
</dl>
{{if eq .State "CREATED"}}
<button onclick="pay(1)">支付</button>
<button onclick="pay(0)">支付，但不通知商户（模拟回调丢失）</button>
{{end}}
<div id="result"></div>
<script>
async function pay(notify){
  const res = await fetch(location.pathname + "/pay?notify=" + notify, {method: "POST"});
  document.getElementById("result").textContent = JSON.stringify(await res.json(), null, 2);
  setTimeout(() => location.reload(), 1200);
}
</script>
</body></html>`))

type pageData struct {
	*service.Payment
	Yuan      string
	ExpiresAt string
}

func (h *handler) page(r *ghttp.Request) {
	p, err := h.provider.Query(r.Context(), r.Get("paymentId").String())
	if err != nil {
		r.Response.WriteStatus(statusOf(err), message(err))
		return
	}
	data := pageData{Payment: p, Yuan: fmt.Sprintf("%d.%02d", p.AmountCents/100, p.AmountCents%100)}
	if p.ExpiresAt != nil {
		data.ExpiresAt = p.ExpiresAt.Local().Format("2006-01-02 15:04:05")
	}
	r.Response.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := pageTemplate.Execute(r.Response.Writer, data); err != nil {
		g.Log().Warningf(r.Context(), "render checkout page: %v", err)
	}
}

func (h *handler) pay(r *ghttp.Request) {
	notify := r.Get("notify", "1").String() != "0"
	p, err := h.provider.Pay(r.Context(), r.Get("paymentId").String(), notify)
	if err != nil {
		r.Response.WriteStatus(statusOf(err))
		r.Response.WriteJson(g.Map{"ok": false, "message": message(err)})
		return
	}
	g.Log().Infof(r.Context(), "payment %s paid (order %s, tx %s, notify %v)", p.ID, p.OrderNo, p.TxID, notify)
	r.Response.WriteJson(g.Map{"ok": true, "paymentId": p.ID, "state": p.State, "providerTxId": p.TxID, "notify": notify})
}

func statusOf(err error) int {
	switch {
	case errors.Is(err, service.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, service.ErrFailedPrecondition), errors.Is(err, service.ErrExpired):
		return http.StatusConflict
	case errors.Is(err, service.ErrInvalidArgument):
		return http.StatusBadRequest
	}
	return http.StatusInternalServerError
}

func message(err error) string {
	switch {
	case errors.Is(err, service.ErrNotFound):
		return "支付单不存在"
	case errors.Is(err, service.ErrFailedPrecondition):
		return "支付单已关闭，不能支付"
	case errors.Is(err, service.ErrExpired):
		return "支付单已过期"
	case errors.Is(err, service.ErrInvalidArgument):
		return "参数错误"
	}
	return "内部错误"
}
