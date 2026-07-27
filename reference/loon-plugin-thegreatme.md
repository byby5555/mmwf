---
type: reference
title: Loon Plugin — TheGreatMe Premium Unlock
---

# TheGreatMe Premium Unlock (Loon Plugin)

**仓库**: https://github.com/Leslie159357/loon-plugin (用户自己的 loon-plugin 仓库)

**插件路径**: `plugins/TheGreatMe/TheGreatMe_Unlock.loon.plugin`

**来源项目**: [tgm-crack/TGM-Unlock](https://github.com/tgm-crack/TGM-Unlock)

## 功能

RevenueCat Bypass，解锁 TheGreatMe 全部高级功能。

## 技术原理

通过 MITM 拦截 RevenueCat API 请求，用脚本修改订阅响应，实现伪激活。

### MITM 域名
- `*.revenuecat.com`
- `*.rc-backend.com`
- `api-paywalls.revenuecat.com`
- `api-diagnostics.revenuecat.com`

### 拦截脚本

```
http-response ^https:\/\/api\.revenuecat\.com\/v1\/(subscribers|offerings|product_entitlement_mapping)
  → script: tgm_premium.js
http-response ^https:\/\/api-paywalls\.revenuecat\.com\/
  → script: tgm_premium.js
```

### 脚本来源
`https://raw.githubusercontent.com/tgm-crack/TGM-Unlock/main/tgm_premium.js`

[Source: GitHub raw, 2026-07-17]
