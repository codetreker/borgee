package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"borgee-server/internal/auth"
	"borgee-server/internal/config"
	"borgee-server/internal/store"

	"github.com/google/uuid"
)

type contextKey string

const requestIDKey contextKey = "requestID"

func RequestIDFromContext(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDKey).(string); ok {
		return id
	}
	return ""
}

func recoverMiddleware(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				logger.Error("panic recovered",
					"error", fmt.Sprintf("%v", err),
					"stack", string(debug.Stack()),
					"path", r.URL.Path,
				)
				writeErrorResponse(w, http.StatusInternalServerError, "Internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := uuid.New().String()
		w.Header().Set("X-Request-ID", id)
		ctx := context.WithValue(r.Context(), requestIDKey, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (sr *statusRecorder) WriteHeader(code int) {
	sr.status = code
	sr.ResponseWriter.WriteHeader(code)
}

func (sr *statusRecorder) Unwrap() http.ResponseWriter {
	return sr.ResponseWriter
}

func (sr *statusRecorder) Flush() {
	if flusher, ok := sr.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func loggerMiddleware(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		logger.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration", time.Since(start).String(),
			"request_id", RequestIDFromContext(r.Context()),
		)
	})
}

// isLoopbackOrigin reports whether origin is an http/https URL whose host is a
// loopback address (localhost / 127.0.0.1 / ::1), on ANY port. The host match
// is exact (u.Hostname()), so lookalikes like "localhost.evil.com",
// "notlocalhost" or "127.0.0.1.evil.com" are rejected — no suffix/prefix/Contains
// matching. Unparseable origins and non-http(s) schemes return false.
//
// This is the dev-mode allowlist that replaced blind Origin reflection (#1023):
// reflecting an arbitrary Origin while also emitting Allow-Credentials let any
// third-party page read authenticated responses cross-origin.
func isLoopbackOrigin(origin string) bool {
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}
	switch u.Hostname() {
	case "localhost", "127.0.0.1", "::1":
		return true
	default:
		return false
	}
}

// corsMiddleware emits Access-Control-Allow-Origin + Access-Control-Allow-Credentials
// only for an *allowed* request Origin — the two headers are always set together,
// never the credentials flag alone (#1023). An origin is allowed when:
//   - prod (!isDev): it exactly equals the configured allowedOrigin.
//   - dev: it equals allowedOrigin OR is a loopback origin (localhost/127.0.0.1/::1,
//     any port). Dev no longer reflects arbitrary origins.
//
// The vite dev stack proxies same-origin (browser → :5173 → server), and a direct
// cross-origin dev client (e.g. :5174 → :4900) is loopback, so both stay allowed.
func corsMiddleware(isDev bool, allowedOrigin string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		allowed := origin != "" && origin == allowedOrigin
		if !allowed && isDev {
			allowed = isLoopbackOrigin(origin)
		}

		if allowed {
			// ACAO echoes the specific origin (not "*") and is paired with
			// credentials — only ever emitted together for an allowed origin.
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}

		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type,Authorization,X-Dev-User-Id,Last-Event-ID")
		w.Header().Set("Access-Control-Max-Age", "86400")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// contentSecurityPolicy is the single-line CSP served on every response
// (#1108 frontend-F4). Notes on the directive choices:
//   - style-src needs 'unsafe-inline': index.html/admin.html ship an inline
//     <style> block and emoji-mart injects styles at runtime.
//   - script-src 'self' suffices: vite emits external hashed module scripts,
//     no inline <script>.
//   - img-src https: covers user-supplied artifact images — KEEP IN SYNC with
//     frontend-F5's image policy.
//   - media-src 'self' blob: https:: mirrors img-src so external video_link
//     artifacts (<video src="https://...">, MediaPreview.tsx) render.
//   - object-src 'self' https:: pdf_link renders <embed type="application/pdf">
//     (MediaPreview.tsx), governed by object-src; 'none' would block ALL PDF
//     previews. Modern browsers sandbox the built-in PDF viewer (no
//     Flash/plugin risk), so allowing same-origin + https embeds is acceptable.
//   - connect-src lists ws: wss: for the same-origin WebSocket.
const contentSecurityPolicy = "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob: https:; font-src 'self'; connect-src 'self' ws: wss:; media-src 'self' blob: https:; worker-src 'self' blob:; manifest-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'; object-src 'self' https:"

// permissionsPolicy disables browser feature APIs the app never uses.
const permissionsPolicy = "accelerometer=(), camera=(), microphone=(), geolocation=(), gyroscope=(), magnetometer=(), payment=(), usb=(), interest-cohort=()"

func securityHeadersMiddleware(isDev bool, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Content-Security-Policy", contentSecurityPolicy)
		w.Header().Set("Permissions-Policy", permissionsPolicy)
		// HSTS only outside development: dev runs over plain HTTP, and pinning
		// HSTS on localhost would wedge browsers onto https://localhost.
		if !isDev {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		next.ServeHTTP(w, r)
	})
}

type rateLimiter struct {
	mu       sync.Mutex
	clients  map[string]*clientBucket
	authRate float64
	authMax  float64
	userRate float64
	userMax  float64
	anonRate float64
	anonMax  float64
}

type clientBucket struct {
	tokens   float64
	max      float64
	rate     float64
	lastTime time.Time
}

func newRateLimiter(ctx context.Context, cfg *config.Config) *rateLimiter {
	rl := &rateLimiter{
		clients:  make(map[string]*clientBucket),
		authRate: float64(cfg.RateLimitAuthPerSec),
		authMax:  float64(cfg.RateLimitAuthBurst),
		userRate: float64(cfg.RateLimitUserPerSec),
		userMax:  float64(cfg.RateLimitUserBurst),
		anonRate: float64(cfg.RateLimitAnonPerSec),
		anonMax:  float64(cfg.RateLimitAnonBurst),
	}
	go rl.cleanup(ctx)
	return rl
}

func (rl *rateLimiter) cleanup(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			// TEST-FIX-2: ctx-aware shutdown. Tests that pass t.Context()
			// (Go 1.24+ auto-cancels on test end) get clean goroutine exit
			// instead of leaked ticker firing on closed DB.
			return
		case <-ticker.C:
			rl.evictStale(time.Now())
		}
	}
}

// evictStale removes client buckets that haven't been touched in 10+ minutes.
// Extracted from cleanup() so the eviction logic is unit-testable without
// waiting on the 5-minute ticker.
func (rl *rateLimiter) evictStale(now time.Time) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	for key, b := range rl.clients {
		if now.Sub(b.lastTime) > 10*time.Minute {
			delete(rl.clients, key)
		}
	}
}

// allow 实际 token-bucket 取一个 token. rate / max 由外面按桶选好传进来.
// key 在外面构造前缀 (auth:<ip> / user:<userID> / anon:<ip>) 防字面值撞.
func (rl *rateLimiter) allow(key string, rate, max float64) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	b, ok := rl.clients[key]
	if !ok {
		b = &clientBucket{tokens: max, max: max, rate: rate, lastTime: time.Now()}
		rl.clients[key] = b
	} else {
		// 桶参数变了 (热配置 / 测试覆盖) 顺手刷新
		b.rate = rate
		b.max = max
	}

	now := time.Now()
	elapsed := now.Sub(b.lastTime).Seconds()
	b.tokens += elapsed * b.rate
	if b.tokens > b.max {
		b.tokens = b.max
	}
	b.lastTime = now

	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// isAuthPath 判 path 是否走 auth 桶 (防爆破): /api/v1/auth/* + /admin-api/auth/*.
func isAuthPath(p string) bool {
	return strings.HasPrefix(p, "/api/v1/auth/") || strings.HasPrefix(p, "/admin-api/auth/")
}

// rateLimitMiddleware enforces three-tier token-bucket throttling.
//
// 桶选择 (优先级 auth > user > anon):
//   - auth: path 前缀 /api/v1/auth/ 或 /admin-api/auth/, key=auth:<ip>, 即便登录用户也走这桶 (防爆破)
//   - user: 通过 AuthenticateFlexible 拿到 user, key=user:<userID>
//   - anon: 其它, key=anon:<ip>
//
// 中间件链上 rate limit 套在 mux 外, auth 中间件在 mux 内 per-route,
// 所以 UserFromContext 在这里拿不到. 此处自己 AuthenticateFlexible 一次轻量
// 探测 — token 无效就 fall back IP 桶, 不 panic.
//
// E2E bypass (两道门, environment-gated): isDevelopment=true AND
// X-E2E-Test:1 时跳过限速. 单门不算 (header 在 prod 可伪造, dev mode 单门
// 会让本地浏览器流量静默 bypass 掩盖真客户端 bug).
func rateLimitMiddleware(rl *rateLimiter, s *store.Store, cfg *config.Config, next http.Handler) http.Handler {
	isDevelopment := cfg.IsDevelopment()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isDevelopment && r.Header.Get("X-E2E-Test") == "1" {
			next.ServeHTTP(w, r)
			return
		}

		ip := clientIP(r, cfg.TrustedProxyCount)

		var key string
		var rate, max float64
		switch {
		case isAuthPath(r.URL.Path):
			key = "auth:" + ip
			rate, max = rl.authRate, rl.authMax
		default:
			if u := auth.AuthenticateFlexible(s, cfg, r); u != nil {
				key = "user:" + u.ID
				rate, max = rl.userRate, rl.userMax
				// 把已鉴权 user 塞 ctx, 让 mux 内的 auth.AuthMiddleware 短路,
				// 省一次 GetUserByID DB lookup per 登录请求.
				r = r.WithContext(auth.ContextWithUser(r.Context(), u))
			} else {
				key = "anon:" + ip
				rate, max = rl.anonRate, rl.anonMax
			}
		}

		if !rl.allow(key, rate, max) {
			writeErrorResponse(w, http.StatusTooManyRequests, "Rate limit exceeded")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// clientIP derives the per-IP rate-limit key from the request, honoring a
// configurable trusted-proxy count (#1108 F2).
//
//   - trustedProxyCount <= 0 (the safe default): use host(RemoteAddr) only and
//     completely ignore X-Forwarded-For / X-Real-IP. Both are client-controlled
//     and forgeable, so trusting them lets an unauthenticated attacker rotate a
//     fresh `auth:<ip>` bucket per request and bypass login brute-force throttling.
//   - trustedProxyCount = N (≥1): trust the rightmost N hops of
//     `chain = X-Forwarded-For ++ [host(RemoteAddr)]` as proxies and pick the
//     real client just left of them: chain[len-1-N] (lower-bound clamped to 0).
//     XFF entries an attacker injects on the left fall outside the trusted
//     window and are ignored. X-Real-IP is no longer special-cased (equally
//     spoofable — it was a second bypass path).
func clientIP(r *http.Request, trustedProxyCount int) string {
	remote := hostOnly(r.RemoteAddr)
	if trustedProxyCount <= 0 {
		return remote
	}

	chain := make([]string, 0, 4)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		for _, part := range strings.Split(xff, ",") {
			if v := strings.TrimSpace(part); v != "" {
				chain = append(chain, v)
			}
		}
	}
	chain = append(chain, remote)

	idx := len(chain) - 1 - trustedProxyCount
	if idx < 0 {
		idx = 0
	}
	return chain[idx]
}

// hostOnly strips the port from a host:port address (e.g. RemoteAddr).
func hostOnly(addr string) string {
	if idx := strings.LastIndex(addr, ":"); idx != -1 {
		return addr[:idx]
	}
	return addr
}
