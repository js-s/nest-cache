package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthzWithoutDatabase(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	response := httptest.NewRecorder()

	(application{}).healthz(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusServiceUnavailable)
	}
}
