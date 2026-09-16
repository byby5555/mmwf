// v3_routing.go — v3u/v3 op 分发框架。
//
// op 路由映射表从官方前端 bundle (162 参照机下发的 assets/index-*.js) 逆向生成,
// 共 267 个唯一 op 码。完整清单见 .temp/_op_routing_map.txt。
//
// 分层策略:
//   第一层: 已实现的 op → 直接映射到现有 handler 路径
//   第二层: 未实现 op  → 返回占位响应 (success:true, data:null), 让前端不崩溃
//
// 符号约定: ¤ = U+00A4 (user/user+admin 混合), § = U+00A7 (admin 为主)

package handler

import (
	"net/http"
	"net/url"
	"sync"
)

// HTTP method 常量别名 (缩短映射表书写)
const (
	v3GET    = http.MethodGet
	v3POST   = http.MethodPost
	v3PUT    = http.MethodPut
	v3DELETE = http.MethodDelete
	v3PATCH  = http.MethodPatch
)

// V3Target 是一个 op 对应的内部路由目标。
type V3Target struct {
	Method string
	Path   string
}

// v3UserRoutes: /api/v3u 的 op → 路由 (¤ 前缀 op, 用户侧)
var v3UserRoutes = map[string]V3Target{
	// --- 登录/认证 (登录前流程) ---
	"¤d8617d022a414fd0": {v3POST, "/api/login"},
	"¤a31d3f4f81bdbb7e": {v3GET, "/api/setup/status"},
	"¤82bd8d71746f1694": {v3POST, "/api/setup"},
	"¤602549901192a110": {v3GET, "/api/captcha/config"},
	"¤e659c6cd1e9b15af": {v3POST, "/api/domain-check"},
	"¤8eff17ea3b98abd6": {v3POST, "/api/login/2fa/recovery"},
	"¤932a9149769e9ffb": {v3POST, "/api/login/2fa/verify"},

	// --- 用户核心 ---
	"¤ba8ad99c09e3549e": {v3GET, "/api/user/profile"},
	"¤f03140ec862a3778": {v3GET, "/api/user/config"},
	"¤c297bcbaa22132e9": {v3PUT, "/api/user/config"},
	"¤4e17a2ea1eae2f78": {v3GET, "/api/user/token"},
	"¤7db4904acbba8324": {v3POST, "/api/user/token/regenerate"},
	"¤76be241e26965f78": {v3PUT, "/api/user/token"},
	"¤749915ca02ff68ac": {v3POST, "/api/user/password"},
	"¤add70c9ed64935de": {v3GET, "/api/user/subscriptions"},
	"¤c1b0ede30e8cb370": {v3GET, "/api/user/package-assignments"},
	"¤f616b30ab24ed987": {v3GET, "/api/user/forward-nodes"},
	"¤ff8804c1e493810f": {v3GET, "/api/user/forward-nodes"},
	"¤a14fe83e4851bb18": {v3GET, "/api/user/routed-outbounds"},
	"¤8730410b8d1f6fc0": {v3GET, "/api/user/2fa/status"},
	"¤ed0ab02c1ed8a767": {v3POST, "/api/user/2fa/enable"},
	"¤c8518db672c88609": {v3POST, "/api/user/2fa/disable"},
	"¤2538e7ac038b08d9": {v3GET, "/api/traffic-summary"},
	"¤2033390a35b9e02e": {v3GET, "/api/user/telegram-binding"},
	"¤847da72b1ee38d10": {v3POST, "/api/user/telegram-binding/create"},
	"¤tab2e36664d12dee9": {v3DELETE, "/api/user/telegram-binding/delete"},
	"¤056367cb172aa187": {v3POST, "/api/user/renewal/submit"},
	"¤3ac79622892fe222": {v3GET, "/api/user/renewal-request"},
	"¤b0152fcd392f3a06": {v3PUT, "/api/user/profile/update"},
	"¤1338289308c8db86": {v3POST, "/api/2fa/verify"},

	// --- 模板 ---
	"¤147319194b493bb4": {v3GET, "/api/user/default-template"},
	"¤db790da69751971e": {v3PUT, "/api/user/default-template/set"},

	// --- 公开 ---
	"¤c646fd4b5ac6b72b": {v3GET, "/api/public/refetch-interval"},
	"¤fff0c6677d0e5bd4": {v3GET, "/api/branding"},

	// --- admin ops 混入 (¤ 前缀但实际是 admin 端点, admin 登录后可经 v3u 发) ---
	"¤030e76b59a09e459": {v3POST, "/api/admin/api-tokens/create"},
	"¤06fe64b7b96ae168": {v3POST, "/api/admin/external-sync/import"},
	"¤10b317f626a031fc": {v3GET, "/api/admin/api-token"},
	"¤4eb5d58779eb6e34": {v3POST, "api/admin/routed-outbounds/create"},
	"¤677266bc6d7c2ff8": {v3POST, "/api/user/forward/create"},
	"¤adf4e861f21d5408": {v3POST, "/api/admin/external-sync/select"},
	"¤ae8f44d8b265a6c5": {v3GET, "/api/admin/proxy-provider-configs"},
	"¤b6b143a1593924ba": {v3POST, "/api/admin/proxy-provider-configs/create"},
	"¤a6a184282b27df62": {v3POST, "/api/admin/subscribe-files/preview"},
	"¤ea2cf021140a96eb": {v3GET, "/api/admin/proxy-group-categories"},
}

// v3AdminRoutes: /api/v3 的 op → 路由 (§ 前缀 op)
var v3AdminRoutes = map[string]V3Target{
	// 用户管理 — /api/admin/users GET 列表 + POST 动作 (payload 内 action 字段区分,
	// 见 users.go / mmwf_fresh 的现有实现)。多个 op 共享同一路径, 动作在 payload 中。
	"§03f3e85b3feaae55": {v3POST, "/api/admin/users"},
	"§5a8525db88308ba9": {v3POST, "/api/admin/users"},
	"§66811e391bb8c52d": {v3POST, "/api/admin/users"},
	"§89bf7f4e7afb5b90": {v3POST, "/api/admin/users"},
	"§939ed195d70303a3": {v3POST, "/api/admin/users"},
	"§9827bede7148e683": {v3POST, "/api/admin/users"},
	"§984718ccf66fbae7": {v3POST, "/api/admin/users"},
	"§b5c31fab7c9e92f8": {v3POST, "/api/admin/users"},
	"§bd8e679ab4130ab0": {v3POST, "/api/admin/users"},
	"§c54c289c1f6b8352": {v3POST, "/api/admin/users"},
	"§cfb75e2e437f9ac1": {v3POST, "/api/admin/users"},
	"§ef5c012ed717a2d2": {v3POST, "/api/admin/users"},
	"§c8383d4437b6ddff": {v3POST, "/api/admin/users"},
	"§99cc845bbc10f1ec": {v3PUT, "/api/admin/users"},
	"§b7194dfabc229019": {v3PUT, "/api/admin/users"},
	"§f47eb450071f7043": {v3PUT, "/api/admin/users"},
	"§b6f8c98d4b8f1c70": {v3GET, "/api/admin/users"},
	"§f555801cf00bd100": {v3DELETE, "/api/admin/users"},

	// 节点管理
	"§b02ec184f40f46f3": {v3GET, "/api/admin/nodes"},
	"§87069e81b99750b3": {v3POST, "/api/admin/nodes/normalize"},
	"§914b57fe83b5c5cf": {v3POST, "/api/admin/nodes/import"},
	"§994f8315797ab697": {v3POST, "/api/admin/nodes/batch-delete"},
	"§a9032ee81f812843": {v3POST, "/api/admin/nodes/delete"},
	"§a9c7e29900e7e941": {v3POST, "/api/admin/nodes/rename"},
	"§ed699dfa96d688a1": {v3POST, "/api/admin/nodes/update"},
	"§25e0cf8cacb34efb": {v3POST, "/api/admin/nodes/probe"},
	"§69e57ddaa8d7dc42": {v3POST, "/api/admin/nodes/batch-probe"},
	"§0e49bf03631c89fb": {v3POST, "/api/admin/nodes/generate-link"},
	"§1e5a52a405eb9f25": {v3POST, "/api/admin/nodes/fetch-url"},
	"§4fbde412961e348b": {v3POST, "/api/admin/nodes/parse-content"},

	// 远程服务器
	"§b16e74baa40c6a77": {v3GET, "/api/admin/remote-servers"},
	"§ecc3464be116181c": {v3PUT, "/api/admin/remote-servers/update"},
	"§30ce8ad43219ce97": {v3POST, "/api/admin/remote-servers/delete"},

	// 订阅文件
	"§cce38361e0ab6820": {v3GET, "/api/admin/subscribe-files"},
	"§28b2db0b3d098bc6": {v3POST, "/api/admin/subscribe-files"},
	"§270ee7b84b86817d": {v3PUT, "/api/admin/subscribe-files"},
	"§25d490345d5b787a": {v3DELETE, "/aapi/admin/certificates"},

	// 订阅模板
	"§b0c68e7bd88510c3": {v3GET, "/api/admin/templates"},
	"§d7aade6b83f868d5": {v3POST, "/api/admin/templates/apply"},
	"§96f20dab326c5f02": {v3POST, "/api/admin/templates/preview"},
	"§47b927f4f57ddc14": {v3POST, "/api/admin/templates/parse"},
	"§c1f999de86f21f35": {v3POST, "/api/admin/templates/rename"},
	"§5002f68653e44eec": {v3POST, "/api/admin/templates/save"},
	"§1732a9645baf2104": {v3PUT, "/api/admin/default-template"},
	"§df4763ad40808e15": {v3GET, "/api/admin/default-template"},
	"§114ce6bf92480cd0": {v3PUT, "/api/admin/rule-templates/visibility"},

	// 系统设置
	"§1d0cdb7046335717": {v3GET, "/api/admin/branding"},
	"§34d865d8754bf54a": {v3POST, "/api/admin/branding"},
	"§1d7335cef7bbfbbe": {v3PUT, "/api/admin/update-cdn-enabled"},
	"§36ad798e97b8eb75": {v3GET, "/api/admin/update-cdn-enabled"},
	"§6ad59db52d1a7555": {v3GET, "/api/admin/tgbot-settings"},
	"§6fc4d93f1494ff17": {v3PUT, "/api/admin/tgbot-settings/update"},
	"§af7bfc337ce5d5f0": {v3PUT, "/api/admin/default-theme-settings"},
	"§b8dc91f573b49bce": {v3GET, "/api/admin/default-theme-settings"},
	"§1513a9c4775e7503": {v3PUT, "/api/admin/redeem-template"},
	"§18fb86405e2ac501": {v3GET, "/api/admin/redeem-template"},
	"§2201c75d3d30989d": {v3PUT, "/api/admin/system-intervals"},
	"§e27c1db7d09c6552": {v3GET, "/api/admin/system-intervals"},
	"§19708c2f6572f113": {v3PUT, "/api/admin/user-permissions-config/update"},
	"§e8e4f79ba0371c33": {v3GET, "/api/admin/user-permissions-config"},
	"§12483c1b0adaaca3": {v3PUT, "/api/admin/miaomiaowu-features/update"},
	"§cacc77d224f60fca": {v3GET, "/api/admin/miaomiaowu-features"},
	"§c85aaaae883b16c0": {v3PUT, "/api/admin/require-encryption"},
	"§6e8a40e20eb15fb3": {v3GET, "/api/admin/require-encryption"},
	"§70c8a6048196fa74": {v3GET, "/api/admin/silent-mode"},
	"§eed187fcadec562e": {v3PUT, "/api/admin/silent-mode"},
}

// 分发器挂载点 — 由 main.go 注入内部 mux (含 audit/silent-mode 中间件链),
// 避免 handler 包对 main 的循环依赖。
var (
	v3DispatchMu      sync.RWMutex
	v3DispatchHandler http.Handler
)

// SetV3Dispatcher 安装内部路由分发器。
func SetV3Dispatcher(h http.Handler) {
	v3DispatchMu.Lock()
	v3DispatchHandler = h
	v3DispatchMu.Unlock()
}

// ResolveV3Op 查 op 路由映射表。isAdmin=true 走 admin 表, false 走 user 表。
func ResolveV3Op(op string, isAdmin bool) (V3Target, bool) {
	if isAdmin {
		t, ok := v3AdminRoutes[op]
		return t, ok
	}
	t, ok := v3UserRoutes[op]
	return t, ok
}

// V3Dispatch 把内部请求喂给完整 mux。
func V3Dispatch(w http.ResponseWriter, r *http.Request) {
	v3DispatchMu.RLock()
	h := v3DispatchHandler
	v3DispatchMu.RUnlock()
	if h == nil {
		http.Error(w, "v3 dispatcher not installed", http.StatusInternalServerError)
		return
	}
	h.ServeHTTP(w, r)
}

// parseURLForV3 解析路由路径 (含 query)。
func parseURLForV3(raw string) *url.URL {
	u, err := url.Parse(raw)
	if err != nil {
		return &url.URL{Path: "/"}
	}
	return u
}
