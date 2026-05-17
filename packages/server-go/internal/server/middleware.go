package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
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

func corsMiddleware(isDev bool, allowedOrigin string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		if isDev {
			if origin != "" {
				w.Header().Set("Access-Control-Allow-Origin", origin)
			}
		} else {
			if origin == allowedOrigin {
				w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
			}
		}

		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type,Authorization,X-Dev-User-Id,Last-Event-ID")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Max-Age", "86400")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
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

		ip := clientIP(r)

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

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if parts := strings.SplitN(xff, ",", 2); len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	// Strip port from RemoteAddr
	addr := r.RemoteAddr
	if idx := strings.LastIndex(addr, ":"); idx != -1 {
		return addr[:idx]
	}
	return addr
}
