package session

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const cookieName = "tgifreezeday_session"

// SetUserID writes a signed session cookie containing the user ID.
func SetUserID(w http.ResponseWriter, secret []byte, userID int64) {
	val := sign(secret, strconv.FormatInt(userID, 10))
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    val,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int((30 * 24 * time.Hour).Seconds()),
	})
}

// GetUserID reads and verifies the session cookie. Returns (userID, true) on success.
func GetUserID(r *http.Request, secret []byte) (int64, bool) {
	cookie, err := r.Cookie(cookieName)
	if err != nil {
		return 0, false
	}
	parts := strings.SplitN(cookie.Value, ".", 2)
	if len(parts) != 2 {
		return 0, false
	}
	payload, sig := parts[0], parts[1]

	expected := sign(secret, payload)
	expectedParts := strings.SplitN(expected, ".", 2)
	if len(expectedParts) != 2 || !hmac.Equal([]byte(sig), []byte(expectedParts[1])) {
		return 0, false
	}

	id, err := strconv.ParseInt(payload, 10, 64)
	if err != nil {
		return 0, false
	}
	return id, true
}

// Clear removes the session cookie.
func Clear(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:   cookieName,
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})
}

func sign(secret []byte, payload string) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(payload))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf("%s.%s", payload, sig)
}
