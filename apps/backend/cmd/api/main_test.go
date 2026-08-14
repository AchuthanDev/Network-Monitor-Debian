package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDashboardDoesNotReturnFakeMetrics(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard", nil)
	response := httptest.NewRecorder()

	writeDashboard(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.Code)
	}
	if !strings.Contains(response.Body.String(), "unavailable") {
		t.Fatalf("dashboard response should report unavailable metrics, got %s", response.Body.String())
	}
}
