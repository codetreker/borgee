// Package api — pwa_manifest.go: DL-4.4 PWA installable Web App Manifest
// endpoint (must-fix 收口).
//
// Blueprint锚: docs/blueprint/current/client-shape.md L22 ("Mobile PWA + Web Push
// VAPID") + L42 ("manifest + install prompt + Web Push + standalone")
// + L46 (实现路径: manifest.json + push subscription endpoint + VAPID).
// Spec: docs/implementation/modules/dl-4-spec.md §1 DL-4.4.
//
// Endpoint surface:
//   - GET /api/v1/pwa/manifest    PWA Web App Manifest JSON (W3C
//                                  https://www.w3.org/TR/appmanifest/)
//
// Content-Type: application/manifest+json (W3C 标准 MIME, 浏览器 install
// prompt 严格识别).
//
// ⚠️ 命名拆死锚 — 跟 HB-1 #491 `GET /api/v1/plugin-manifest` 拆开:
//   - 本 endpoint: PWA installable web app manifest (浏览器 install
//     prompt 用), HTTPS + 公开无 auth.
//   - HB-1 endpoint: install-butler 消费 binary plugin manifest (双签
//     必需, 蓝图 host-bridge §1.2 + §4.5 "未签 100% reject").
//   - 反向 grep `manifest/plugins|plugin-manifest` 在本文件 count==0.
//
// 反约束 (DL-4 spec §0):
//   - **公开 endpoint** — 不走 authMw (浏览器 install prompt 在 login
//     前 fetch; manifest 内容不含 PII / secret).
//   - 不挂 secret 字段 (跟 web_push_subscriptions 反约束同源).
//   - 内容静态 (server 端常量, 不查 DB) — 避免无 auth endpoint 触发 DB
//     load.
package api

import (
	"encoding/json"
	"net/http"
)

// PWAManifestHandler serves the PWA Web App Manifest at GET
// /api/v1/pwa/manifest. Wired in server.go boot.
type PWAManifestHandler struct{}

// RegisterRoutes wires the public GET endpoint (no authMw — browser
// fetches before login at install prompt time).
func (h *PWAManifestHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/pwa/manifest", h.handleGet)
}

// pwaManifestPayload mirrors the W3C App Manifest spec subset Borgee
// commits to:
//
//   - name / short_name: install prompt + 主屏 label
//   - start_url: 桌面图标点击进 SPA 根
//   - display: standalone (蓝图 L22 字面)
//   - theme_color / background_color: install prompt 配色
//   - scope: navigation scope (/= 全应用)
//   - icons: 多尺寸 PNG (192x192 / 512x512 W3C 推荐基线)
//
// 反约束: 字段集严闭, 不挂 client_id / id / secrets / orientation 等
// 跟蓝图 L22 三件套 不直接相关字面.
type pwaManifestPayload struct {
	Name            string             `json:"name"`
	ShortName       string             `json:"short_name"`
	StartURL        string             `json:"start_url"`
	Display         string             `json:"display"`
	ThemeColor      string             `json:"theme_color"`
	BackgroundColor string             `json:"background_color"`
	Scope           string             `json:"scope"`
	Icons           []pwaManifestIcon  `json:"icons"`
}

type pwaManifestIcon struct {
	Src     string `json:"src"`
	Sizes   string `json:"sizes"`
	Type    string `json:"type"`
	Purpose string `json:"purpose,omitempty"`
}

// pwaManifestPayloadStatic is the source-of-truth manifest content.
// Static (no DB load) — safe to serve from public endpoint.
//
// 字面 byte-identical 跟 蓝图 client-shape.md L22 (Mobile PWA + standalone)
// + L46 (manifest 静态文件); icons 引用 packages/client/public/icons/
// 现有 SVG 资源 (跟 packages/client/public/manifest.json 静态文件同源).
var pwaManifestPayloadStatic = pwaManifestPayload{
	Name:            "Borgee",
	ShortName:       "Borgee",
	StartURL:        "/",
	Display:         "standalone", // 蓝图 L22 字面
	ThemeColor:      "#16213e",    // 跟 manifest.json 静态文件 byte-identical
	BackgroundColor: "#1a1a2e",
	Scope:           "/",
	Icons: []pwaManifestIcon{
		{
			Src:     "/icons/icon-192.svg",
			Sizes:   "192x192",
			Type:    "image/svg+xml",
			Purpose: "any",
		},
		{
			Src:     "/icons/icon-512.svg",
			Sizes:   "512x512",
			Type:    "image/svg+xml",
			Purpose: "any",
		},
		{
			Src:     "/favicon.svg",
			Sizes:   "any",
			Type:    "image/svg+xml",
			Purpose: "any maskable",
		},
	},
}

// handleGet serves the W3C-compliant PWA Web App Manifest.
//
// Content-Type: application/manifest+json (W3C 标准 MIME, 浏览器 install
// prompt 识别 trigger).
//
// 反约束: 不返 secret, 不查 DB, 不依赖 auth context.
func (h *PWAManifestHandler) handleGet(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/manifest+json")
	// Cache hint — manifest static content. 1h browser cache.
	w.Header().Set("Cache-Control", "public, max-age=3600")
	if err := json.NewEncoder(w).Encode(pwaManifestPayloadStatic); err != nil {
		http.Error(w, "manifest encode failed", http.StatusInternalServerError)
		return
	}
}
