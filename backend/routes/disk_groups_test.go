package routes_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"capacitarr/internal/db"
	"capacitarr/internal/testutil"
)

func TestUpdateDiskGroup_WithOverride(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	group := db.DiskGroup{
		MountPath:    "/mnt/media",
		TotalBytes:   1000000000000, // 1 TB
		UsedBytes:    800000000000,
		ThresholdPct: 85.0,
		TargetPct:    75.0,
	}
	if err := database.Create(&group).Error; err != nil {
		t.Fatalf("Failed to create disk group: %v", err)
	}

	body := `{"thresholdPct": 90, "targetPct": 80, "totalBytesOverride": 500000000000}`
	req := testutil.AuthenticatedRequest(t, http.MethodPut, fmt.Sprintf("/api/disk-groups/%d", group.ID), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var updated db.DiskGroup
	if err := json.Unmarshal(rec.Body.Bytes(), &updated); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if updated.ThresholdPct != 90 {
		t.Errorf("expected threshold 90, got %f", updated.ThresholdPct)
	}
	if updated.TargetPct != 80 {
		t.Errorf("expected target 80, got %f", updated.TargetPct)
	}
	if updated.TotalBytesOverride == nil || *updated.TotalBytesOverride != 500000000000 {
		t.Errorf("expected override 500000000000, got %v", updated.TotalBytesOverride)
	}
}

func TestUpdateDiskGroup_ClearOverride(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	override := int64(500000000000)
	group := db.DiskGroup{
		MountPath:          "/mnt/media",
		TotalBytes:         1000000000000,
		UsedBytes:          800000000000,
		TotalBytesOverride: &override,
		ThresholdPct:       85.0,
		TargetPct:          75.0,
	}
	if err := database.Create(&group).Error; err != nil {
		t.Fatalf("Failed to create disk group: %v", err)
	}

	// Send null to clear the override
	body := `{"thresholdPct": 85, "targetPct": 75, "totalBytesOverride": null}`
	req := testutil.AuthenticatedRequest(t, http.MethodPut, fmt.Sprintf("/api/disk-groups/%d", group.ID), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var updated db.DiskGroup
	if err := json.Unmarshal(rec.Body.Bytes(), &updated); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if updated.TotalBytesOverride != nil {
		t.Errorf("expected override nil after clear, got %v", updated.TotalBytesOverride)
	}
}

func TestUpdateDiskGroup_NegativeOverrideRejected(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	group := db.DiskGroup{
		MountPath:    "/mnt/media",
		TotalBytes:   1000000000000,
		UsedBytes:    800000000000,
		ThresholdPct: 85.0,
		TargetPct:    75.0,
	}
	if err := database.Create(&group).Error; err != nil {
		t.Fatalf("Failed to create disk group: %v", err)
	}

	body := `{"thresholdPct": 85, "targetPct": 75, "totalBytesOverride": -100}`
	req := testutil.AuthenticatedRequest(t, http.MethodPut, fmt.Sprintf("/api/disk-groups/%d", group.ID), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("Expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateDiskGroup_WithoutOverride(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	group := db.DiskGroup{
		MountPath:    "/mnt/media",
		TotalBytes:   1000000000000,
		UsedBytes:    800000000000,
		ThresholdPct: 85.0,
		TargetPct:    75.0,
	}
	if err := database.Create(&group).Error; err != nil {
		t.Fatalf("Failed to create disk group: %v", err)
	}

	// Omit totalBytesOverride entirely — should still work
	body := `{"thresholdPct": 90, "targetPct": 80}`
	req := testutil.AuthenticatedRequest(t, http.MethodPut, fmt.Sprintf("/api/disk-groups/%d", group.ID), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var updated db.DiskGroup
	if err := json.Unmarshal(rec.Body.Bytes(), &updated); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if updated.ThresholdPct != 90 {
		t.Errorf("expected threshold 90, got %f", updated.ThresholdPct)
	}
}

func TestListDiskGroups_IncludesOverride(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	override := int64(500000000000)
	database.Create(&db.DiskGroup{
		MountPath: "/mnt/a", TotalBytes: 1000000000000, UsedBytes: 500000000000,
		TotalBytesOverride: &override, ThresholdPct: 85, TargetPct: 75,
	})
	database.Create(&db.DiskGroup{
		MountPath: "/mnt/b", TotalBytes: 2000000000000, UsedBytes: 1000000000000,
		ThresholdPct: 85, TargetPct: 75,
	})

	req := testutil.AuthenticatedRequest(t, http.MethodGet, "/api/disk-groups", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var groups []db.DiskGroup
	if err := json.Unmarshal(rec.Body.Bytes(), &groups); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if len(groups) != 2 {
		t.Fatalf("Expected 2 groups, got %d", len(groups))
	}

	// First group should have override
	if groups[0].TotalBytesOverride == nil || *groups[0].TotalBytesOverride != 500000000000 {
		t.Errorf("expected first group override 500000000000, got %v", groups[0].TotalBytesOverride)
	}
	// Second group should have no override
	if groups[1].TotalBytesOverride != nil {
		t.Errorf("expected second group no override, got %v", groups[1].TotalBytesOverride)
	}
}
