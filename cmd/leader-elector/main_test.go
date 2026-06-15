package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWebHandlerReportsLeaderName(t *testing.T) {
	leaders.Set("pod-a")
	t.Cleanup(func() { leaders.Set("") })

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	res := httptest.NewRecorder()

	webHandler(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
	}
	if got, want := res.Body.String(), `{"name":"pod-a"}`; got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}
}
