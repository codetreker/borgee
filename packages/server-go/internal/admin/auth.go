// Package admin implements the ADM-0 admin auth path — completely independent
// from internal/auth (the user auth path).
//
// Blueprint: admin-model §1.2 (option-B environment bootstrap, no promote path) + §3
// (separate admins table). Implementation R3 PR #189 lays out the 3 sub-PRs
// (ADM-0.1 / 0.2 / 0.3); this file lands in ADM-0.1 and is extended in ADM-0.2 to back the
// cookie value with a server-side `admin_sessions` row instead of carrying
// the raw admin id.
//
// Hard constraints (review checklist §ADM-0.1 + §ADM-0.2):
//   - cookie name MUST be the literal `borgee_admin_session`
//   - cookie value MUST be an opaque session token, never a raw admin id
//     (ADM-0.2: server-side session lookup, not parse)
//   - this package MUST NOT import borgee-server/internal/auth (auth path
//     isolation; grep for that import path must come up empty)
//   - bcrypt verify MUST use subtle.ConstantTimeCompare wrapping
//     bcrypt.CompareHashAndPassword's success/failure semantics
//   - admins table fields are strict: id / login / password_hash / created_at
//
// ADM-0.2 cuts the legacy `internal/api/admin_auth.go` JWT path and the
// `/api/v1/admin/*` user-rail god-mode mount; only `borgee_admin_session`
// cookie + `/admin-api/v1/*` survive.
package admin

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"borgee-server/internal/idgen"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// CookieName is the literal cookie name for the new admin auth path.
// Locked by blueprint admin-model §1.2 and review checklist §ADM-0.1.
// DO NOT change this value — tests assert the exact string.
const CookieName = "borgee_admin_session"

// Env var names are locked literals from the review checklist.
const (
	EnvAdminLogin        = "BORGEE_ADMIN_LOGIN"
	EnvAdminPasswordHash = "BORGEE_ADMIN_PASSWORD_HASH"
	// EnvTestFastAdminPassword is an explicit e2e-only shortcut for admin login
	// checks. It does not relax bootstrap: the stored admin hash must still be
	// a valid bcrypt hash with cost >= MinBcryptCost.
	EnvTestFastAdminPassword = "BORGEE_TEST_FAST_ADMIN_PASSWORD"
	// EnvAdminPassword (ADMIN-PASSWORD-PLAIN-ENV option B): plaintext password env.
	// At startup, bcrypt.GenerateFromPassword hashes it before storing it in
	// memory; this code never writes the plaintext to disk. It is mutually
	// exclusive with EnvAdminPasswordHash, and setting both panics to avoid
	// surprising precedence. Prefer HASH in production (env leaks expose only the
	// hash); use PLAIN in development/testing to simplify deployment.
	EnvAdminPassword = "BORGEE_ADMIN_PASSWORD"
)

// SessionTTL is the lifetime of an admin session cookie. v0: 7 days.
const SessionTTL = 7 * 24 * time.Hour

// MinBcryptCost mirrors the review checklist requirement that
// password_hash be a bcrypt hash with cost ≥ 10.
const MinBcryptCost = 10

// SessionTokenBytes is the length (in raw bytes) of the random portion of an
// admin session token. 32 bytes → 64 hex chars; ample for a v0 sweep table.
const SessionTokenBytes = 32

// Admin is the in-package row shape for the `admins` table created by
// migration adm_0_1_admins (v=4).
type Admin struct {
	ID           string `gorm:"primaryKey;column:id"`
	Login        string `gorm:"column:login;uniqueIndex"`
	PasswordHash string `gorm:"column:password_hash"`
	CreatedAt    int64  `gorm:"column:created_at"`
}

// TableName pins the GORM table mapping to the literal name created by the
// migration; we deliberately do not rely on naming conventions here.
func (Admin) TableName() string { return "admins" }

// AdminSession backs the v=5 `admin_sessions` table (ADM-0.2). Cookie value
// is `Token` (opaque); resolution requires a row lookup, never parsing.
type AdminSession struct {
	Token     string `gorm:"primaryKey;column:token"`
	AdminID   string `gorm:"column:admin_id;index"`
	CreatedAt int64  `gorm:"column:created_at"`
	ExpiresAt int64  `gorm:"column:expires_at;index"`
}

// TableName pins the literal name created by the migration.
func (AdminSession) TableName() string { return "admin_sessions" }

// FindByLogin returns the admin row matching login or (nil, nil) if none.
// All other DB errors propagate.
func FindByLogin(db *gorm.DB, login string) (*Admin, error) {
	var a Admin
	err := db.Where("login = ?", login).Take(&a).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// VerifyPassword checks plaintext against the stored bcrypt hash. The bcrypt
// library already runs in constant time relative to its input length, but per
// review checklist §1.E we additionally wrap the success signal in
// subtle.ConstantTimeCompare so timing of the boolean result cannot leak.
func VerifyPassword(hash, plain string) bool {
	if expected := os.Getenv(EnvTestFastAdminPassword); expected != "" {
		return subtle.ConstantTimeCompare([]byte(plain), []byte(expected)) == 1
	}
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain))
	ok := byte(0)
	if err == nil {
		ok = 1
	}
	// Compare the boolean signal against the constant 1 in constant time.
	return subtle.ConstantTimeCompare([]byte{ok}, []byte{1}) == 1
}

// CreateSession inserts a new admin_sessions row and returns the opaque token
// to be set as the cookie value. ADM-0.2 invariant: token is never derived
// from admin id; resolution requires a row lookup.
func CreateSession(db *gorm.DB, adminID string, now time.Time) (string, error) {
	buf := make([]byte, SessionTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("admin session: rand: %w", err)
	}
	token := hex.EncodeToString(buf)
	row := AdminSession{
		Token:     token,
		AdminID:   adminID,
		CreatedAt: now.UnixMilli(),
		ExpiresAt: now.Add(SessionTTL).UnixMilli(),
	}
	if err := db.Create(&row).Error; err != nil {
		return "", err
	}
	return token, nil
}

// ResolveSession looks up the session row by token and returns the admin if
// the row exists, is unexpired, and the joined admins row exists. Otherwise
// returns (nil, nil); errors propagate only on DB failure.
func ResolveSession(db *gorm.DB, token string, now time.Time) (*Admin, error) {
	if token == "" {
		return nil, nil
	}
	var s AdminSession
	err := db.Where("token = ?", token).Take(&s).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if s.ExpiresAt <= now.UnixMilli() {
		return nil, nil
	}
	var a Admin
	err = db.Where("id = ?", s.AdminID).Take(&a).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// DeleteSession revokes a single session token (logout).
func DeleteSession(db *gorm.DB, token string) error {
	if token == "" {
		return nil
	}
	return db.Where("token = ?", token).Delete(&AdminSession{}).Error
}

// DeleteSessionsForAdmin revokes all sessions for a given admin id. Used by
// ADM-0.3 backfill to invalidate legacy admin cookies.
func DeleteSessionsForAdmin(db *gorm.DB, adminID string) error {
	return db.Where("admin_id = ?", adminID).Delete(&AdminSession{}).Error
}

// Handler exposes the admin auth endpoints under /admin-api/auth/*.
type Handler struct {
	DB     *gorm.DB
	Logger *slog.Logger
	// IsDevelopment toggles cookie Secure flag selection (dev allows http).
	IsDevelopment bool
	// Now is injectable for deterministic tests; defaults to time.Now.
	Now func() time.Time
}

func (h *Handler) now() time.Time {
	if h.Now != nil {
		return h.Now()
	}
	return time.Now()
}

// RegisterRoutes wires the admin auth surface (login / logout / me).
// ADM-0.1 introduced /admin-api/auth/login (no /v1/) alongside the legacy
// /admin-api/v1/auth/login JWT route in internal/api. ADM-0.2 retires the
// JWT route and remounts /admin-api/v1/auth/* on this same handler so the
// admin SPA continues to call /admin-api/v1/auth/login unchanged — the
// difference is that the cookie value is now an opaque session token, not
// a JWT.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /admin-api/auth/login", h.handleLogin)
	mux.HandleFunc("POST /admin-api/auth/logout", h.handleLogout)
	mux.Handle("GET /admin-api/auth/me", RequireAdmin(h.DB, h.now)(http.HandlerFunc(h.handleMe)))
	// v1 aliases (admin SPA path stability across ADM-0.1 → 0.2 cutover).
	mux.HandleFunc("POST /admin-api/v1/auth/login", h.handleLogin)
	mux.HandleFunc("POST /admin-api/v1/auth/logout", h.handleLogout)
	mux.Handle("GET /admin-api/v1/auth/me", RequireAdmin(h.DB, h.now)(http.HandlerFunc(h.handleMe)))
}

type loginRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

func (h *Handler) handleLogin(w http.ResponseWriter, r *http.Request) {
	var body loginRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if body.Login == "" || body.Password == "" {
		writeJSONError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	admin, err := FindByLogin(h.DB, body.Login)
	if err != nil {
		if h.Logger != nil {
			h.Logger.Error("admin login lookup failed", "error", err)
		}
		writeJSONError(w, http.StatusInternalServerError, "lookup failed")
		return
	}

	// Always run bcrypt verify (even on missing row) to preserve constant-time
	// shape — feed a bogus hash so timing is comparable to the hit path.
	hash := ""
	if admin != nil {
		hash = admin.PasswordHash
	} else {
		hash = "$2a$10$0000000000000000000000000000000000000000000000000000"
	}
	if !VerifyPassword(hash, body.Password) || admin == nil {
		writeJSONError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// ADM-0.2: cookie value MUST be an opaque session token, never the
	// raw admin id. CreateSession inserts an admin_sessions row.
	token, err := CreateSession(h.DB, admin.ID, h.now())
	if err != nil {
		if h.Logger != nil {
			h.Logger.Error("admin session create failed", "error", err)
		}
		writeJSONError(w, http.StatusInternalServerError, "session create failed")
		return
	}

	cookie := &http.Cookie{
		Name:     CookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(SessionTTL.Seconds()),
	}
	if !h.IsDevelopment {
		host := strings.Split(r.Host, ":")[0]
		if host != "localhost" && host != "127.0.0.1" {
			cookie.Secure = true
		}
	}
	http.SetCookie(w, cookie)

	writeJSON(w, http.StatusOK, map[string]any{
		"id":    admin.ID,
		"login": admin.Login,
	})
}

func (h *Handler) handleLogout(w http.ResponseWriter, r *http.Request) {
	token := ""
	if c, err := r.Cookie(CookieName); err == nil {
		token = c.Value
	}
	if token != "" {
		if err := DeleteSession(h.DB, token); err != nil && h.Logger != nil {
			h.Logger.Warn("admin logout: delete session failed", "error", err)
		}
	}
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) handleMe(w http.ResponseWriter, r *http.Request) {
	a := AdminFromContext(r.Context())
	if a == nil {
		writeJSONError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":    a.ID,
		"login": a.Login,
	})
}

// AdminFromRequest looks up the admin associated with the request's
// `borgee_admin_session` cookie via admin_sessions table. Returns (nil, nil)
// when no cookie is set, the session is missing/expired, or the admin is gone.
func AdminFromRequest(db *gorm.DB, r *http.Request) (*Admin, error) {
	c, err := r.Cookie(CookieName)
	if err != nil || c.Value == "" {
		return nil, nil
	}
	return ResolveSession(db, c.Value, time.Now())
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeJSONError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"error": msg})
}

// Bootstrap inserts (or no-ops on conflict) the env-configured admin into the
// admins table. Called from cmd/collab/main.go startup.
//
// fail-loud: missing env vars panic with a clear message (review checklist
// §ADM-0.1 §1.A). This is deliberate — admin auth must not silently degrade.
//
// Idempotent: re-runs with the same login do nothing (review checklist §1.B).
// The migration's UNIQUE(login) index is the source of truth; this function
// uses INSERT … ON CONFLICT DO NOTHING semantics.
func Bootstrap(db *gorm.DB) error {
	login := os.Getenv(EnvAdminLogin)
	hash := os.Getenv(EnvAdminPasswordHash)
	plain := os.Getenv(EnvAdminPassword)
	return BootstrapWith(db, login, hash, plain)
}

// BootstrapWith is the testable form of Bootstrap. cmd/collab uses Bootstrap;
// tests inject explicit values.
//
// ADMIN-PASSWORD-PLAIN-ENV option B: choose exactly one of hash or plain.
//   - hash != "" + plain == "": existing path, use the stored bcrypt hash directly.
//   - hash == "" + plain != "": new path, hash plain in memory with
//     bcrypt.GenerateFromPassword.
//   - hash != "" + plain != "": panic because the two inputs are mutually exclusive.
//   - hash == "" + plain == "": panic because one input is required.
func BootstrapWith(db *gorm.DB, login, hash, plain string) error {
	if login == "" {
		panic(fmt.Sprintf("admin bootstrap: %s is required (set to the admin login)", EnvAdminLogin))
	}
	if hash != "" && plain != "" {
		panic(fmt.Sprintf("admin bootstrap: %s and %s are mutually exclusive (二选一); set exactly one", EnvAdminPasswordHash, EnvAdminPassword))
	}
	if hash == "" && plain == "" {
		panic(fmt.Sprintf("admin bootstrap: either %s (bcrypt hash, cost ≥ %d) or %s (plain text, hashed at startup) is required", EnvAdminPasswordHash, MinBcryptCost, EnvAdminPassword))
	}

	// PLAIN path: hash plaintext at startup with MinBcryptCost. The result is
	// only ever held in memory + persisted to admins table once (idempotent
	// INSERT below); the env var itself is never written to disk by this
	// function. Operators are warned in docs that env leakage = plain leakage.
	if plain != "" {
		generated, err := bcrypt.GenerateFromPassword([]byte(plain), MinBcryptCost)
		if err != nil {
			panic(fmt.Sprintf("admin bootstrap: bcrypt hash of %s failed: %v", EnvAdminPassword, err))
		}
		hash = string(generated)
	}

	// Reject obviously non-bcrypt or low-cost hashes. The review checklist locks
	// "password_hash 不是 bcrypt (明文 / sha256 / md5)" and "bcrypt cost ≥ 10".
	cost, err := bcrypt.Cost([]byte(hash))
	if err != nil {
		panic(fmt.Sprintf("admin bootstrap: %s is not a valid bcrypt hash: %v", EnvAdminPasswordHash, err))
	}
	if cost < MinBcryptCost {
		panic(fmt.Sprintf("admin bootstrap: %s bcrypt cost %d < required %d", EnvAdminPasswordHash, cost, MinBcryptCost))
	}

	// Idempotent insert: ON CONFLICT(login) DO NOTHING via the UNIQUE index.
	row := Admin{
		ID:           idgen.NewID(),
		Login:        login,
		PasswordHash: hash,
		CreatedAt:    time.Now().UnixMilli(),
	}
	res := db.Exec(
		`INSERT INTO admins (id, login, password_hash, created_at)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(login) DO NOTHING`,
		row.ID, row.Login, row.PasswordHash, row.CreatedAt,
	)
	return res.Error
}
