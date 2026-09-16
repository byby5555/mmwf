package handler

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"sync"
	"time"

	"miaomiaowux/internal/notify"
)

// ============ server notify throttle ============

// 服务器上下线通知去抖动 — 同一 server 同一状态短时间内频繁触发(国际线路抖动 / 心跳延迟卡阈值 /
// 多条代码路径重复上报)会 spam 用户 telegram。这里维护 per-(事件类型, server) 上次通知时间,
// 窗口内重复的"同 server + 同事件"(连续 online 或连续 offline)被吞掉。
// 关键:online 与 offline 各自独立 throttle —— 一次真实的"离线 → 恢复"两条都会照常发。
var (
	serverNotifyMu       sync.Mutex
	serverNotifyLastSent = make(map[string]time.Time) // key = 事件类型|serverName
)

func serverNotifyThrottleInterval() time.Duration {
	if v := os.Getenv("MMWX_SERVER_NOTIFY_THROTTLE_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			if n <= 0 {
				return 0
			}
			return time.Duration(n) * time.Second
		}
	}
	return 5 * time.Minute
}

func shouldThrottleServerNotify(serverName string, event notify.EventType) bool {
	interval := serverNotifyThrottleInterval()
	if interval <= 0 {
		return false
	}
	key := string(event) + "|" + serverName
	serverNotifyMu.Lock()
	defer serverNotifyMu.Unlock()
	if last, ok := serverNotifyLastSent[key]; ok && time.Since(last) < interval {
		return true
	}
	serverNotifyLastSent[key] = time.Now()
	return false
}

// ============ notifyAsync ============

// notifyAsync 是所有 tg 通知的统一入口,集中三件事:
//  1. nil Notifier(未初始化 / 配置加载失败)→ throttled log "notifier_nil"
//  2. CheckEnabled 拒绝(全局关 / token 空 / chatID 空 / 该事件开关 off)→ throttled log 对应 reason
//  3. 实际 send 错误(TG API 429/5xx / 网络) → log
func notifyAsync(ctx context.Context, t notify.EventType, title, msg string) {
	n := GetNotifier()
	if n == nil {
		logNotifyReasonThrottled(t, "notifier_nil")
		return
	}
	ok, reason := n.CheckEnabled(t)
	if !ok {
		logNotifyReasonThrottled(t, string(reason))
		return
	}
	go func() {
		if err := n.Send(ctx, notify.Event{Type: t, Title: title, Message: msg}); err != nil {
			log.Printf("[Notify] send failed event=%s: %v", t, err)
		}
	}()
}

var notifyLogMu sync.Mutex
var notifyLogLast = make(map[string]time.Time)

func logNotifyReasonThrottled(t notify.EventType, reason string) {
	key := string(t) + ":" + reason
	notifyLogMu.Lock()
	defer notifyLogMu.Unlock()
	if last, ok := notifyLogLast[key]; ok && time.Since(last) < 5*time.Minute {
		return
	}
	notifyLogLast[key] = time.Now()
	log.Printf("[Notify] skip event=%s reason=%s (5min log throttle to avoid spam)", t, reason)
}

// ============ Send* helper functions ============

func SendServerOnlineNotification(ctx context.Context, serverName, ip string) {
	if shouldThrottleServerNotify(serverName, notify.EventServerOnline) {
		log.Printf("[Notify] server=%q online suppressed by throttle (within %s window)", serverName, serverNotifyThrottleInterval())
		return
	}
	notifyAsync(ctx, notify.EventServerOnline,
		"🟢 服务器上线",
		fmt.Sprintf("服务器: `%s`\nIP: `%s`", serverName, ip),
	)
}

func SendServerOfflineNotification(ctx context.Context, serverName, ip string) {
	if shouldThrottleServerNotify(serverName, notify.EventServerOffline) {
		log.Printf("[Notify] server=%q offline suppressed by throttle (within %s window)", serverName, serverNotifyThrottleInterval())
		return
	}
	notifyAsync(ctx, notify.EventServerOffline,
		"🔴 服务器离线",
		fmt.Sprintf("服务器: `%s`\nIP: `%s`", serverName, ip),
	)
}

func SendXrayStatusChangeNotification(ctx context.Context, serverName, ip string, running bool) {
	if running {
		notifyAsync(ctx, notify.EventServerOnline,
			"🟢 Xray 已启动",
			fmt.Sprintf("服务器: `%s`\nIP: `%s`", serverName, ip),
		)
	} else {
		notifyAsync(ctx, notify.EventServerOffline,
			"🔴 Xray 已停止",
			fmt.Sprintf("服务器: `%s`\nIP: `%s`", serverName, ip),
		)
	}
}

func SendLoginNotification(ctx context.Context, username, ip string) {
	notifyAsync(ctx, notify.EventLogin,
		"用户登录",
		fmt.Sprintf("用户: `%s`\nIP: `%s`", username, ip),
	)
}

func SendSubscribeFetchNotification(ctx context.Context, username, clientType, ip string) {
	notifyAsync(ctx, notify.EventSubscribeFetch,
		"订阅获取",
		fmt.Sprintf("用户: `%s`\n客户端: `%s`\nIP: `%s`", username, clientType, ip),
	)
}

func SendTrafficThreshold80Notification(ctx context.Context, username string, usedGB, limitGB float64) {
	notifyAsync(ctx, notify.EventTrafficThreshold80,
		"⚠️ 用户流量预警 80%",
		fmt.Sprintf("用户: `%s`\n已用: %.2fGB / %.0fGB (≥80%%)", username, usedGB, limitGB),
	)
}

func SendOverLimitNotification(ctx context.Context, username string, usedGB, limitGB float64) {
	notifyAsync(ctx, notify.EventOverLimit,
		"🚫 用户流量超限",
		fmt.Sprintf("用户: `%s`\n已用: %.2fGB / %.0fGB (100%%+)\n→ 已从入站移除", username, usedGB, limitGB),
	)
}

func SendPackageExpiringNotification(ctx context.Context, username, pkgName string, daysLeft int, endDate string) {
	notifyAsync(ctx, notify.EventPackageExpiring,
		"⏰ 套餐即将到期",
		fmt.Sprintf("用户: `%s`\n套餐: `%s`\n到期: %s (剩 %d 天)", username, pkgName, endDate, daysLeft),
	)
}

func SendPackageExpiredNotification(ctx context.Context, username, pkgName string) {
	notifyAsync(ctx, notify.EventPackageExpired,
		"❌ 套餐已到期",
		fmt.Sprintf("用户: `%s`\n套餐: `%s`\n→ 已清除入站/路由分配", username, pkgName),
	)
}

func SendUserRegisteredNotification(ctx context.Context, username, email, source string) {
	notifyAsync(ctx, notify.EventUserRegistered,
		"🆕 新用户注册",
		fmt.Sprintf("用户: `%s`\n邮箱: `%s`\n来源: %s", username, email, source),
	)
}

func SendTelegramBoundNotification(ctx context.Context, username string, tgID int64, tgHandle string) {
	notifyAsync(ctx, notify.EventTelegramBound,
		"🔗 用户已绑定 Telegram",
		fmt.Sprintf("用户: `%s`\nTG ID: `%d`\nTG: @%s", username, tgID, tgHandle),
	)
}

func SendCertResultNotification(ctx context.Context, domain string, success bool, detail string) {
	if success {
		notifyAsync(ctx, notify.EventCertResult,
			"✅ 证书申请成功",
			fmt.Sprintf("域名: `%s`\n%s", domain, detail),
		)
	} else {
		notifyAsync(ctx, notify.EventCertResult,
			"❗️ 证书申请失败",
			fmt.Sprintf("域名: `%s`\n错误: %s", domain, detail),
		)
	}
}

func SendAgentLongOfflineNotification(ctx context.Context, serverName, ip string, offlineMinutes int) {
	notifyAsync(ctx, notify.EventAgentLongOffline,
		"⏱ Agent 长期离线",
		fmt.Sprintf("服务器: `%s`\nIP: `%s`\n已离线: %d 分钟", serverName, ip, offlineMinutes),
	)
}

func SendConnLimitExceededNotification(ctx context.Context, username, nodeName string, delta int) {
	msg := fmt.Sprintf("用户: `%s`\n本周期内新连接被拒: %d 次", username, delta)
	if nodeName != "" {
		msg = fmt.Sprintf("用户: `%s`\n节点: `%s`\n本周期内新连接被拒: %d 次", username, notify.EscapeMarkdown(nodeName), delta)
	}
	notifyAsync(ctx, notify.EventDeviceLimitExceeded, "🔌 连接数超限", msg)
}
