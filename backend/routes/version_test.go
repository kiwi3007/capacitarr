package routes_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"capacitarr/internal/db"
	"capacitarr/internal/testutil"
	"capacitarr/routes"
)

// ---------- GET /api/version/check ----------

func TestVersionCheck_DisabledByPreference(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	// Disable update checks in preferences
	if err := database.Model(&db.PreferenceSet{}).Where("id = 1").Update("check_for_updates", false).Error; err != nil {
		t.Fatalf("Failed to update preferences: %v", err)
	}

	req := testutil.AuthenticatedRequest(t, http.MethodGet, "/api/version/check", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Current         string `json:"current"`
		Latest          string `json:"latest"`
		UpdateAvailable bool   `json:"updateAvailable"`
		ReleaseURL      string `json:"releaseUrl"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp.UpdateAvailable {
		t.Error("Expected updateAvailable to be false when checks are disabled")
	}
	if resp.Latest != "" {
		t.Errorf("Expected empty latest version when checks are disabled, got %q", resp.Latest)
	}
	if resp.ReleaseURL != "" {
		t.Errorf("Expected empty releaseUrl when checks are disabled, got %q", resp.ReleaseURL)
	}
}

func TestVersionCheck_ReturnsCurrentVersion(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	req := testutil.AuthenticatedRequest(t, http.MethodGet, "/api/version/check", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Current string `json:"current"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp.Current != "v0.0.0-test" {
		t.Errorf("Expected current version 'v0.0.0-test', got %q", resp.Current)
	}
}

// ---------- compareSemver ----------

func TestCompareSemver(t *testing.T) {
	tests := []struct {
		name     string
		a, b     string
		expected int
	}{
		{"equal versions", "1.0.0", "1.0.0", 0},
		{"patch higher", "1.0.1", "1.0.0", 1},
		{"patch lower", "1.0.0", "1.0.1", -1},
		{"stable > prerelease", "1.0.0", "1.0.0-rc.1", 1},
		{"prerelease higher", "1.0.0-rc.3", "1.0.0-rc.1", 1},
		{"major higher", "2.0.0", "1.9.9", 1},
		{"with v prefix", "v1.0.0", "v1.0.0", 0},
		{"mixed v prefix", "v1.0.1", "1.0.0", 1},
		{"prerelease < stable", "1.0.0-rc.1", "1.0.0", -1},
		{"minor higher", "1.1.0", "1.0.9", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := routes.CompareSemverForTest(tt.a, tt.b)
			if got != tt.expected {
				t.Errorf("compareSemver(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.expected)
			}
		})
	}
}
