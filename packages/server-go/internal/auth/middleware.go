package auth

import (
	"context"
	"net/http"
	"strings"

	"borgee-server/internal/config"
	"borgee-server/internal/store"

	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const userContextKey contextKey = "auth_user"

// CookieName is the user-rail session cookie literal 单一来源 (ADM-0.1 +
// COOKIE-NAME-CLEANUP). Mirror of admin-rail `internal/admin/auth.go::
// CookieName="borgee_admin_session"`. Keep the literal value here; refactor
// callsites to use this const so any future rename touches one line.
//
// 约定: 改 cookie 字面值 = 全用户 session 失效 — 0 字面值改 (本 milestone
// 仅引用 单一来源, 不改值). 反向 grep `"borgee_token"` in production .go (除本
// const) ==0 hit (post-cleanup).
const CookieName = "borgee_token"

func UserFromContext(ctx context.Context) *store.User {
	if u, ok := ctx.Value(userContextKey).(*store.User); ok {
		return u
	}
	return nil
}

func setUserContext(r *http.Request, user *store.User) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), userContextKey, user))
}

// ContextWithUser injects an authenticated user into the request context.
// Used by tests that drive handlers directly without the live mux's auth
// middleware. Mirrors setUserContext but exported for cross-package use.
func ContextWithUser(ctx context.Context, user *store.User) context.Context {
	return context.WithValue(ctx, userContextKey, user)
}

type Claims struct {
	UserID string `json:"userId"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

func AuthMiddleware(s *store.Store, cfg *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 0. 上游 (rate-limit 中间件) 已鉴权过的话 user 在 ctx 里,
			// 短路掉, 省一次 GetUserByID DB lookup. user 不在 ctx 才走下面
			// 真鉴权流程 (鉴权失败原样 401, 不改语义).
			if existing := UserFromContext(r.Context()); existing != nil {
				next.ServeHTTP(w, r)
				return
			}

			// 1. Check cookie
			if cookie, err := r.Cookie(CookieName); err == nil {
				if user := ValidateJWT(s, cfg.JWTSecret, cookie.Value); user != nil {
					next.ServeHTTP(w, setUserContext(r, user))
					return
				}
			}

			// 2. Check Authorization: Bearer <api_key>
			if authHeader := r.Header.Get("Authorization"); strings.HasPrefix(authHeader, "Bearer ") {
				apiKey := strings.TrimPrefix(authHeader, "Bearer ")
				if user, err := s.GetUserByAPIKey(apiKey); err == nil && user.DeletedAt == nil && !user.Disabled {
					next.ServeHTTP(w, setUserContext(r, user))
					return
				}
			}

			// 3. Dev auth bypass
			if cfg.IsDevelopment() && cfg.DevAuthBypass {
				if devUserID := r.Header.Get("X-Dev-User-Id"); devUserID != "" {
					if user, err := s.GetUserByID(devUserID); err == nil {
						next.ServeHTTP(w, setUserContext(r, user))
						return
					}
				}
				// ADM-0.3: fallback to first member (users.role enum collapsed
				// to {'member','agent'}; admin authority is admin-rail only).
				users, err := s.ListUsers()
				if err == nil {
					for i := range users {
						if users[i].Role == "member" {
							next.ServeHTTP(w, setUserContext(r, &users[i]))
							return
						}
					}
				}
			}

			writeJSON401(w)
		})
	}
}

func ValidateJWT(s *store.Store, secret string, tokenStr string) *store.User {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
		return []byte(secret), nil
	})
	if err != nil || !token.Valid {
		return nil
	}

	user, err := s.GetUserByID(claims.UserID)
	if err != nil || user.DeletedAt != nil || user.Disabled {
		return nil
	}
	return user
}

func writeJSON401(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	w.Write([]byte(`{"error":"Unauthorized"}`))
}

func AuthenticateFlexible(s *store.Store, cfg *config.Config, r *http.Request) *store.User {
	if authHeader := r.Header.Get("Authorization"); strings.HasPrefix(authHeader, "Bearer ") {
		apiKey := strings.TrimPrefix(authHeader, "Bearer ")
		if user, err := s.GetUserByAPIKey(apiKey); err == nil && user.DeletedAt == nil && !user.Disabled {
			return user
		}
	}

	if cookie, err := r.Cookie(CookieName); err == nil {
		if user := ValidateJWT(s, cfg.JWTSecret, cookie.Value); user != nil {
			return user
		}
	}

	if cfg.IsDevelopment() && cfg.DevAuthBypass {
		if devUserID := r.Header.Get("X-Dev-User-Id"); devUserID != "" {
			if user, err := s.GetUserByID(devUserID); err == nil {
				return user
			}
		}
	}

	return nil
}

func AuthenticateFromAPIKey(s *store.Store, apiKey string) *store.User {
	if apiKey == "" {
		return nil
	}
	user, err := s.GetUserByAPIKey(apiKey)
	if err != nil || user.DeletedAt != nil || user.Disabled {
		return nil
	}
	return user
}

func AuthenticateFromQuery(s *store.Store, r *http.Request, paramName string) *store.User {
	apiKey := r.URL.Query().Get(paramName)
	return AuthenticateFromAPIKey(s, apiKey)
}
