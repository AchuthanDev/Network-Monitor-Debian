package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	gatewayconfig "github.com/AchuthanDev/Network-Monitor-Debian/features/gateway/config"
)

func TestDashboardDoesNotReturnFakeMetrics(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard", nil)
	response := httptest.NewRecorder()

	writeDashboard(response, request, nil)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.Code)
	}
	if !strings.Contains(response.Body.String(), "unavailable") {
		t.Fatalf("dashboard response should report unavailable metrics, got %s", response.Body.String())
	}
}

func TestGatewayReadinessReportsNotReadyWithoutLANInterface(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/v1/gateway/readiness", nil)
	response := httptest.NewRecorder()

	writeGatewayReadiness(response, request, gatewayconfig.Default())

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.Code)
	}
	body := response.Body.String()
	if !strings.Contains(body, `"ready":false`) {
		t.Fatalf("gateway readiness should not be ready without a LAN interface, got %s", body)
	}
	if !strings.Contains(body, "lan_interface") {
		t.Fatalf("gateway readiness should include lan_interface check, got %s", body)
	}
}

func TestDevicesEndpointDoesNotFabricateClients(t *testing.T) {
	response := httptest.NewRecorder()

	writeDevices(response, gatewayconfig.Default())

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.Code)
	}
	body := response.Body.String()
	if !strings.Contains(body, `"data":[]`) {
		t.Fatalf("devices endpoint should return an empty verified list, got %s", body)
	}
}

func TestISPUsageReportsUnavailableWithoutDatabase(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/v1/isp-usage", nil)
	response := httptest.NewRecorder()

	writeISPUsage(response, request, nil, gatewayconfig.Default())

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.Code)
	}
	if !strings.Contains(response.Body.String(), "database unavailable") {
		t.Fatalf("ISP usage should report unavailable without database, got %s", response.Body.String())
	}
}

func TestGatewayWizardDisablesApplyWithoutLANInterface(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/v1/gateway/wizard", nil)
	response := httptest.NewRecorder()

	writeGatewayWizard(response, request, gatewayconfig.Default())

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.Code)
	}
	body := response.Body.String()
	if !strings.Contains(body, `"apply_ready":false`) {
		t.Fatalf("wizard should disable apply without dedicated LAN, got %s", body)
	}
	if !strings.Contains(body, "Dedicated LAN interface") {
		t.Fatalf("wizard should mention LAN selection, got %s", body)
	}
}

func TestDestinationsDoNotFabricateRowsWithoutDatabase(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/v1/destinations", nil)
	response := httptest.NewRecorder()

	writeDestinations(response, request, nil)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.Code)
	}
	body := response.Body.String()
	if !strings.Contains(body, `"data":[]`) || !strings.Contains(body, "database unavailable") {
		t.Fatalf("destinations should report unavailable empty rows, got %s", body)
	}
}

func TestClassificationCatalogDocumentsPrivacyBoundary(t *testing.T) {
	response := httptest.NewRecorder()

	writeClassificationCatalog(response, httptest.NewRequest(http.MethodGet, "/api/v1/classification/catalog", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.Code)
	}
	body := response.Body.String()
	if !strings.Contains(body, "no_tls_interception") || !strings.Contains(body, "unknown_https") {
		t.Fatalf("classification catalog should expose privacy and unknown categories, got %s", body)
	}
}

func TestHourlyInvestigationValidatesHour(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/v1/investigation/hour?hour=25", nil)
	response := httptest.NewRecorder()

	writeHourlyInvestigation(response, request, nil)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", response.Code)
	}
}
