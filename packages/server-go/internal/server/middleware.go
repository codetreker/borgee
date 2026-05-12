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
	apiRate  float64
	apiMax   float64
}

type clientBucket struct {
	tokens   float64
	max      float64
	rate     float64
	lastTime time.Time
}

func newRateLimiter(ctx context.Context) *rateLimiter {
	rl := &rateLimiter{
		clients:  make(map[string]*clientBucket),
		authRate: 10.0 / 60.0, // 10 per minute
		authMax:  10,
		apiRate:  100.0 / 60.0, // 100 per minute
		apiMax:   100,
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

func (rl *rateLimiter) allow(ip string, isAuth bool) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rate := rl.apiRate
	max := rl.apiMax
	if isAuth {
		rate = rl.authRate
		max = rl.authMax
	}

	key := fmt.Sprintf("%s:%v", ip, isAuth)
	b, ok := rl.clients[key]
	if !ok {
		b = &clientBucket{tokens: max, max: max, rate: rate, lastTime: time.Now()}
		rl.clients[key] = b
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

// rateLimitMiddleware enforces token-bucket throttling per (IP, isAuth) bucket.
//
// E2E bypass (two gates, environment-gated, production-safe):
//
//	The Playwright e2e suite runs all 5 specs serially from 127.0.0.1 and
//	shares a single global API bucket (100/min). Frontend boot per spec
//	(GET /api/v1/users/me, /channels, /me/permissions, /agent_invitations,
//	/ws reconnect, ...) drains the bucket before the suite finishes,
//	manifesting as 429 on `POST /admin-api/auth/login` partway through.
//
//	When the request carries `X-E2E-Test: 1` AND the server is running
//	with `NODE_ENV=development`, we skip rate limiting. Both gates are
//	required: the header alone is forgeable from outside, and NODE_ENV alone
//	would weaken local development hygiene. In production (NODE_ENV != "development"),
//	the header is ignored entirely; the limiter is unmodified.
//
//	playwright.config.ts already injects `X-E2E-Test: 1` into every
//	request (extraHTTPHeaders, see :65) and sets NODE_ENV=development on
//	the e2e server (:88), so no spec changes are needed.
func rateLimitMiddleware(rl *rateLimiter, isDevelopment bool, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isDevelopment && r.Header.Get("X-E2E-Test") == "1" {
			next.ServeHTTP(w, r)
			return
		}

		ip := clientIP(r)
		isAuth := strings.HasPrefix(r.URL.Path, "/api/v1/auth/register")

		if !rl.allow(ip, isAuth) {
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
