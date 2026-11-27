package api

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const sessionCookieName = "zetnix_session"

func newAuthSecret() []byte {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return b
}

func (s *Server) authEnabled() bool {
	return s.cfg.AuthPass != ""
}

func (s *Server) signSession(user string, exp time.Time) string {
	payload := fmt.Sprintf("%s|%d", user, exp.Unix())
	mac := hmac.New(sha256.New, s.authSecret)
	mac.Write([]byte(payload))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." + sig
}

func (s *Server) verifySession(token string) bool {
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		return false
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return false
	}
	sigBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, s.authSecret)
	mac.Write(payloadBytes)
	expected := mac.Sum(nil)
	if !hmac.Equal(sigBytes, expected) {
		return false
	}
	segs := strings.SplitN(string(payloadBytes), "|", 2)
	if len(segs) != 2 {
		return false
	}
	expUnix, err := strconv.ParseInt(segs[1], 10, 64)
	if err != nil {
		return false
	}
	return time.Now().Unix() < expUnix
}

func (s *Server) isAuthenticated(r *http.Request) bool {
	if !s.authEnabled() {
		return true
	}
	// Optional Bearer token access for CLI/API automation
	if authz := r.Header.Get("Authorization"); strings.HasPrefix(authz, "Bearer ") {
		tok := strings.TrimPrefix(authz, "Bearer ")
		if subtle.ConstantTimeCompare([]byte(tok), []byte(s.cfg.AuthPass)) == 1 {
			return true
		}
	}
	c, err := r.Cookie(sessionCookieName)
	if err != nil {
		return false
	}
	return s.verifySession(c.Value)
}

func (s *Server) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"authRequired":  s.authEnabled(),
		"authenticated": s.isAuthenticated(r),
		"user":          s.cfg.AuthUser,
	})
}

func (s *Server) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	if !s.authEnabled() {
		writeJSON(w, http.StatusOK, map[string]bool{"authenticated": true})
		return
	}
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid login request")
		return
	}

	userMatch := subtle.ConstantTimeCompare([]byte(body.Username), []byte(s.cfg.AuthUser)) == 1
	passMatch := subtle.ConstantTimeCompare([]byte(body.Password), []byte(s.cfg.AuthPass)) == 1
	if !userMatch || !passMatch {
		writeError(w, http.StatusUnauthorized, "invalid username or password")
		return
	}

	exp := time.Now().Add(24 * time.Hour)
	isHTTPS := r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    s.signSession(s.cfg.AuthUser, exp),
		Path:     "/",
		Expires:  exp,
		HttpOnly: true,
		Secure:   isHTTPS,
		SameSite: http.SameSiteLaxMode,
	})
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"authenticated": true,
		"user":          s.cfg.AuthUser,
	})
}

func (s *Server) handleAuthLogout(w http.ResponseWriter, _ *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	writeJSON(w, http.StatusOK, map[string]bool{"authenticated": false})
}

func (s *Server) withAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.authEnabled() || !strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}
		if r.URL.Path == "/api/v1/health" || strings.HasPrefix(r.URL.Path, "/api/v1/auth/") {
			next.ServeHTTP(w, r)
			return
		}
		if !s.isAuthenticated(r) {
			writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		next.ServeHTTP(w, r)
	})
}
