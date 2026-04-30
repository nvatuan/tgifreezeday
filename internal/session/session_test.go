package session

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

var testSecret = []byte("test-secret-key-for-unit-tests")

func TestRoundTrip(t *testing.T) {
	w := httptest.NewRecorder()
	SetUserID(w, testSecret, 42, false)

	r := &http.Request{Header: http.Header{"Cookie": w.Result().Header["Set-Cookie"]}}
	id, ok := GetUserID(r, testSecret)
	if !ok {
		t.Fatal("GetUserID returned false, want true")
	}
	if id != 42 {
		t.Fatalf("GetUserID = %d, want 42", id)
	}
}

func TestTamperedSignature(t *testing.T) {
	w := httptest.NewRecorder()
	SetUserID(w, testSecret, 99, false)

	cookies := w.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("no cookie set")
	}

	// Tamper: change the user ID in the payload
	tampered := "100." + cookies[0].Value[len("99."):]
	r := &http.Request{Header: http.Header{}}
	r.AddCookie(&http.Cookie{Name: cookieName, Value: tampered})

	_, ok := GetUserID(r, testSecret)
	if ok {
		t.Fatal("GetUserID returned true with tampered signature, want false")
	}
}

func TestWrongSecret(t *testing.T) {
	w := httptest.NewRecorder()
	SetUserID(w, testSecret, 7, false)

	r := &http.Request{Header: http.Header{"Cookie": w.Result().Header["Set-Cookie"]}}
	_, ok := GetUserID(r, []byte("different-secret"))
	if ok {
		t.Fatal("GetUserID returned true with wrong secret, want false")
	}
}

func TestNoCookie(t *testing.T) {
	r := &http.Request{Header: http.Header{}}
	_, ok := GetUserID(r, testSecret)
	if ok {
		t.Fatal("GetUserID returned true with no cookie, want false")
	}
}

func TestClear(t *testing.T) {
	w := httptest.NewRecorder()
	Clear(w)

	cookies := w.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("no cookie in response")
	}
	if cookies[0].MaxAge != -1 {
		t.Fatalf("Clear: MaxAge = %d, want -1", cookies[0].MaxAge)
	}
}
