package services

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"capacitarr/internal/db"
	"capacitarr/internal/events"
	"capacitarr/internal/integrations"
	"capacitarr/internal/poster"
)

// ─── Mock PosterManager ─────────────────────────────────────────────────────

// mockPosterManager captures uploaded poster data for assertions.
type mockPosterManager struct {
	mu         sync.Mutex
	uploaded   map[string][]byte // nativeID → imageData
	restored   map[string]bool   // nativeID → true if RestorePosterImage called
	failUpload bool              // if true, UploadPosterImage returns an error
}

func newMockPosterManager() *mockPosterManager {
	return &mockPosterManager{
		uploaded: make(map[string][]byte),
		restored: make(map[string]bool),
	}
}

func (m *mockPosterManager) GetPosterImage(_ string) ([]byte, string, error) {
	return nil, "", nil
}

func (m *mockPosterManager) UploadPosterImage(itemID string, imageData []byte, _ string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failUpload {
		return &integrations.NotFoundError{URL: "mock://404"}
	}
	m.uploaded[itemID] = imageData
	return nil
}

func (m *mockPosterManager) RestorePosterImage(itemID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.restored[itemID] = true
	return nil
}

func (m *mockPosterManager) getUploaded(itemID string) ([]byte, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	data, ok := m.uploaded[itemID]
	return data, ok
}

// ─── Test helpers ───────────────────────────────────────────────────────────

func setupPosterOverlayTest(t *testing.T) (*PosterOverlayService, *events.EventBus) {
	t.Helper()
	database := setupTestDB(t)
	bus := events.NewEventBus()
	t.Cleanup(func() { bus.Close() })

	cacheDir := t.TempDir()
	svc, err := NewPosterOverlayService(database, bus, cacheDir)
	if err != nil {
		t.Fatalf("NewPosterOverlayService failed: %v", err)
	}
	return svc, bus
}

// setupPosterOverlayTestFull creates the service, a mock poster manager registered
// in an IntegrationRegistry, a MappingService with a pre-seeded TMDb→native mapping,
// and pre-seeds the poster cache with a test image. Returns everything needed to
// call UpdateOverlay/RestoreOriginal end-to-end.
func setupPosterOverlayTestFull(t *testing.T) (
	svc *PosterOverlayService,
	deps PosterDeps,
	mockMgr *mockPosterManager,
	item db.SunsetQueueItem,
) {
	t.Helper()
	database := setupTestDB(t)
	bus := events.NewEventBus()
	t.Cleanup(func() { bus.Close() })

	cacheDir := t.TempDir()
	svc, err := NewPosterOverlayService(database, bus, cacheDir)
	if err != nil {
		t.Fatalf("NewPosterOverlayService failed: %v", err)
	}

	// Seed FK prerequisites
	integrationID := seedIntegration(t, database)

	// Register mock poster manager with the seeded integration ID
	mockMgr = newMockPosterManager()
	registry := integrations.NewIntegrationRegistry()
	registry.Register(integrationID, mockMgr)

	// Create mapping service and seed a TMDb→native mapping
	mappingSvc := NewMappingService(database, bus)
	tmdbID := 13029 // Firefly TMDb ID
	if err := database.Create(&db.MediaServerMapping{
		TmdbID:        tmdbID,
		IntegrationID: integrationID,
		NativeID:      "plex-12345",
		MediaType:     "show",
		Title:         "Firefly",
	}).Error; err != nil {
		t.Fatalf("Failed to seed mapping: %v", err)
	}

	// Pre-seed the poster cache with a test image so the service skips HTTP download
	testPoster := createTestPosterJPEG(300, 450)
	item = db.SunsetQueueItem{
		MediaName: "Firefly",
		MediaType: "show",
		TmdbID:    &tmdbID,
		PosterURL: "https://example.com/poster.jpg", // not used due to cache hit
	}
	cacheKey := poster.CacheKey(0, tmdbID, "canonical")
	if err := svc.cache.Store(cacheKey, testPoster); err != nil {
		t.Fatalf("Failed to seed poster cache: %v", err)
	}

	deps = PosterDeps{
		Registry: registry,
		Mapping:  mappingSvc,
	}
	return svc, deps, mockMgr, item
}

// createTestPosterJPEG generates a minimal JPEG poster image for testing.
func createTestPosterJPEG(w, h int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{R: 20, G: 30, B: 80, A: 255})
		}
	}
	var buf bytes.Buffer
	_ = jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85})
	return buf.Bytes()
}

// ─── Existing tests ─────────────────────────────────────────────────────────

func TestPosterOverlayService_RestoreAll_EmptyQueue(t *testing.T) {
	svc, bus := setupPosterOverlayTest(t)
	sunset := NewSunsetService(setupTestDB(t), bus)

	restored, err := svc.RestoreAll(sunset, PosterDeps{})
	if err != nil {
		t.Fatalf("RestoreAll failed: %v", err)
	}
	if restored != 0 {
		t.Errorf("Expected 0 restored, got %d", restored)
	}
}

func TestPosterOverlayService_ValidateCache_NoActiveOverlays(t *testing.T) {
	svc, _ := setupPosterOverlayTest(t)
	// Should not panic or error — just a no-op
	svc.ValidateCache()
}

func TestPosterOverlayService_UpdateOverlay_NilTmdbID(t *testing.T) {
	svc, _ := setupPosterOverlayTest(t)

	item := db.SunsetQueueItem{
		MediaName: "Firefly",
		TmdbID:    nil,
	}

	// Should return nil without error when TmdbID is nil
	err := svc.UpdateOverlay(item, 30, "countdown", PosterDeps{})
	if err != nil {
		t.Errorf("Expected nil error for nil TmdbID, got: %v", err)
	}
}

func TestPosterOverlayService_RestoreOriginal_NilTmdbID(t *testing.T) {
	svc, _ := setupPosterOverlayTest(t)

	item := db.SunsetQueueItem{
		MediaName: "Firefly",
		TmdbID:    nil,
	}

	err := svc.RestoreOriginal(item, PosterDeps{})
	if err != nil {
		t.Errorf("Expected nil error for nil TmdbID, got: %v", err)
	}
}

// ─── CountdownBadge: full overlay pipeline ──────────────────────────────────

func TestPosterOverlay_CountdownBadge(t *testing.T) {
	svc, deps, mockMgr, item := setupPosterOverlayTestFull(t)

	err := svc.UpdateOverlay(item, 7, "countdown", deps)
	if err != nil {
		t.Fatalf("UpdateOverlay failed: %v", err)
	}

	// Verify the mock poster manager received an upload
	uploaded, ok := mockMgr.getUploaded("plex-12345")
	if !ok {
		t.Fatal("Expected poster upload to mock manager, got none")
	}
	if len(uploaded) == 0 {
		t.Fatal("Uploaded poster data is empty")
	}

	// Verify the uploaded data is valid JPEG with expected dimensions
	img, err := jpeg.Decode(bytes.NewReader(uploaded))
	if err != nil {
		t.Fatalf("Uploaded poster is not valid JPEG: %v", err)
	}
	bounds := img.Bounds()
	if bounds.Dx() != 300 || bounds.Dy() != 450 {
		t.Errorf("Expected 300x450 poster, got %dx%d", bounds.Dx(), bounds.Dy())
	}
}

// ─── ZeroDayCountdown: edge case where deletion_date is today ───────────────

func TestPosterOverlay_ZeroDayCountdown(t *testing.T) {
	svc, deps, mockMgr, item := setupPosterOverlayTestFull(t)

	// daysRemaining=0 → "Last day" text
	err := svc.UpdateOverlay(item, 0, "countdown", deps)
	if err != nil {
		t.Fatalf("UpdateOverlay with 0 days failed: %v", err)
	}

	uploaded, ok := mockMgr.getUploaded("plex-12345")
	if !ok {
		t.Fatal("Expected poster upload for zero-day countdown, got none")
	}

	// Verify valid JPEG output
	img, err := jpeg.Decode(bytes.NewReader(uploaded))
	if err != nil {
		t.Fatalf("Zero-day poster is not valid JPEG: %v", err)
	}
	bounds := img.Bounds()
	if bounds.Dx() != 300 || bounds.Dy() != 450 {
		t.Errorf("Expected 300x450 poster, got %dx%d", bounds.Dx(), bounds.Dy())
	}
}

// ─── NegativeDays: edge case where deletion date is past ────────────────────

func TestPosterOverlay_NegativeDays(t *testing.T) {
	svc, deps, mockMgr, item := setupPosterOverlayTestFull(t)

	// daysRemaining=-3 → still "Last day" text (no panic/error)
	err := svc.UpdateOverlay(item, -3, "countdown", deps)
	if err != nil {
		t.Fatalf("UpdateOverlay with negative days failed: %v", err)
	}

	uploaded, ok := mockMgr.getUploaded("plex-12345")
	if !ok {
		t.Fatal("Expected poster upload for negative-days countdown, got none")
	}

	if _, err := jpeg.Decode(bytes.NewReader(uploaded)); err != nil {
		t.Fatalf("Negative-days poster is not valid JPEG: %v", err)
	}
}

// ─── SimpleBadge: "Leaving soon" style ──────────────────────────────────────

func TestPosterOverlay_SimpleBadge(t *testing.T) {
	svc, deps, mockMgr, item := setupPosterOverlayTestFull(t)

	err := svc.UpdateOverlay(item, 14, "simple", deps)
	if err != nil {
		t.Fatalf("UpdateOverlay with simple style failed: %v", err)
	}

	uploaded, ok := mockMgr.getUploaded("plex-12345")
	if !ok {
		t.Fatal("Expected poster upload for simple badge, got none")
	}

	img, err := jpeg.Decode(bytes.NewReader(uploaded))
	if err != nil {
		t.Fatalf("Simple badge poster is not valid JPEG: %v", err)
	}
	bounds := img.Bounds()
	if bounds.Dx() != 300 || bounds.Dy() != 450 {
		t.Errorf("Expected 300x450 poster, got %dx%d", bounds.Dx(), bounds.Dy())
	}
}

// ─── LongTitle: very long media name (service should not break) ─────────────

func TestPosterOverlay_LongTitle(t *testing.T) {
	database := setupTestDB(t)
	bus := events.NewEventBus()
	t.Cleanup(func() { bus.Close() })

	cacheDir := t.TempDir()
	svc, err := NewPosterOverlayService(database, bus, cacheDir)
	if err != nil {
		t.Fatalf("NewPosterOverlayService failed: %v", err)
	}

	integrationID := seedIntegration(t, database)
	mockMgr := newMockPosterManager()
	registry := integrations.NewIntegrationRegistry()
	registry.Register(integrationID, mockMgr)

	mappingSvc := NewMappingService(database, bus)
	tmdbID := 99999
	if err := database.Create(&db.MediaServerMapping{
		TmdbID:        tmdbID,
		IntegrationID: integrationID,
		NativeID:      "plex-99999",
		MediaType:     "movie",
		Title:         "Serenity",
	}).Error; err != nil {
		t.Fatalf("Failed to seed mapping: %v", err)
	}

	// Use a long media name that might stress text rendering
	longName := "Serenity: The Complete Extended Director's Cut Ultimate Edition With Bonus Features and Commentary — A Joss Whedon Film"
	testPoster := createTestPosterJPEG(300, 450)
	item := db.SunsetQueueItem{
		MediaName: longName,
		MediaType: "movie",
		TmdbID:    &tmdbID,
		PosterURL: "https://example.com/poster.jpg",
	}

	cacheKey := poster.CacheKey(0, tmdbID, "canonical")
	if err := svc.cache.Store(cacheKey, testPoster); err != nil {
		t.Fatalf("Failed to seed poster cache: %v", err)
	}

	deps := PosterDeps{
		Registry: registry,
		Mapping:  mappingSvc,
	}

	// The overlay text comes from countdownText, not the media name, so the long
	// title doesn't directly affect rendering — but the service must not error.
	err = svc.UpdateOverlay(item, 30, "countdown", deps)
	if err != nil {
		t.Fatalf("UpdateOverlay with long title failed: %v", err)
	}

	uploaded, ok := mockMgr.getUploaded("plex-99999")
	if !ok {
		t.Fatal("Expected poster upload for long-title item, got none")
	}

	if _, err := jpeg.Decode(bytes.NewReader(uploaded)); err != nil {
		t.Fatalf("Long-title poster is not valid JPEG: %v", err)
	}
}

// ─── SavedOverlay: "Saved by popular demand" banner ─────────────────────────

func TestPosterOverlay_SavedBadge(t *testing.T) {
	svc, deps, mockMgr, item := setupPosterOverlayTestFull(t)

	err := svc.UpdateSavedOverlay(item, deps)
	if err != nil {
		t.Fatalf("UpdateSavedOverlay failed: %v", err)
	}

	uploaded, ok := mockMgr.getUploaded("plex-12345")
	if !ok {
		t.Fatal("Expected poster upload for saved badge, got none")
	}

	img, err := jpeg.Decode(bytes.NewReader(uploaded))
	if err != nil {
		t.Fatalf("Saved badge poster is not valid JPEG: %v", err)
	}
	bounds := img.Bounds()
	if bounds.Dx() != 300 || bounds.Dy() != 450 {
		t.Errorf("Expected 300x450 poster, got %dx%d", bounds.Dx(), bounds.Dy())
	}
}

// ─── RestoreOriginal: round-trip overlay then restore ───────────────────────

func TestPosterOverlay_RestoreOriginal(t *testing.T) {
	svc, deps, mockMgr, item := setupPosterOverlayTestFull(t)

	// First apply an overlay
	err := svc.UpdateOverlay(item, 5, "countdown", deps)
	if err != nil {
		t.Fatalf("UpdateOverlay failed: %v", err)
	}

	// Verify overlay was applied
	overlayData, ok := mockMgr.getUploaded("plex-12345")
	if !ok {
		t.Fatal("Expected overlay upload before restore")
	}

	// Now restore the original
	err = svc.RestoreOriginal(item, deps)
	if err != nil {
		t.Fatalf("RestoreOriginal failed: %v", err)
	}

	// Verify the restored poster was uploaded (should differ from the overlay)
	restoredData, ok := mockMgr.getUploaded("plex-12345")
	if !ok {
		t.Fatal("Expected original poster upload after restore")
	}

	// The restored data should be the original test poster (not the overlay)
	// They should differ because the overlay adds a banner
	if bytes.Equal(overlayData, restoredData) {
		t.Error("Restored poster should differ from overlay poster")
	}

	// Verify it's valid JPEG
	if _, err := jpeg.Decode(bytes.NewReader(restoredData)); err != nil {
		t.Fatalf("Restored poster is not valid JPEG: %v", err)
	}
}

// ─── UpdateOverlay with HTTP download (no cache hit) ────────────────────────

func TestPosterOverlay_DownloadsPosterWhenNotCached(t *testing.T) {
	database := setupTestDB(t)
	bus := events.NewEventBus()
	t.Cleanup(func() { bus.Close() })

	cacheDir := t.TempDir()
	svc, err := NewPosterOverlayService(database, bus, cacheDir)
	if err != nil {
		t.Fatalf("NewPosterOverlayService failed: %v", err)
	}

	// Serve a test poster from httptest server.
	// Uses http.ServeContent to avoid semgrep false positive on raw w.Write()
	// (go.lang.security.audit.xss.no-direct-write-to-responsewriter).
	testPoster := createTestPosterJPEG(200, 300)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, "poster.jpg", time.Time{}, bytes.NewReader(testPoster))
	}))
	defer ts.Close()

	integrationID := seedIntegration(t, database)
	mockMgr := newMockPosterManager()
	registry := integrations.NewIntegrationRegistry()
	registry.Register(integrationID, mockMgr)

	mappingSvc := NewMappingService(database, bus)
	tmdbID := 44214 // Serenity
	if err := database.Create(&db.MediaServerMapping{
		TmdbID:        tmdbID,
		IntegrationID: integrationID,
		NativeID:      "plex-44214",
		MediaType:     "movie",
		Title:         "Serenity",
	}).Error; err != nil {
		t.Fatalf("Failed to seed mapping: %v", err)
	}

	item := db.SunsetQueueItem{
		MediaName: "Serenity",
		MediaType: "movie",
		TmdbID:    &tmdbID,
		PosterURL: ts.URL + "/poster.jpg", // served by httptest
	}

	deps := PosterDeps{
		Registry: registry,
		Mapping:  mappingSvc,
	}

	// No cache pre-seeded — service must download from PosterURL
	err = svc.UpdateOverlay(item, 14, "countdown", deps)
	if err != nil {
		t.Fatalf("UpdateOverlay with HTTP download failed: %v", err)
	}

	uploaded, ok := mockMgr.getUploaded("plex-44214")
	if !ok {
		t.Fatal("Expected poster upload after HTTP download, got none")
	}

	img, err := jpeg.Decode(bytes.NewReader(uploaded))
	if err != nil {
		t.Fatalf("Downloaded poster overlay is not valid JPEG: %v", err)
	}
	bounds := img.Bounds()
	if bounds.Dx() != 200 || bounds.Dy() != 300 {
		t.Errorf("Expected 200x300 poster, got %dx%d", bounds.Dx(), bounds.Dy())
	}

	// Verify the original was cached for future use
	cacheKey := poster.CacheKey(0, tmdbID, "canonical")
	if !svc.cache.Has(cacheKey) {
		t.Error("Expected original poster to be cached after download")
	}
}

// ─── UpdateOverlay with empty PosterURL and no cache ────────────────────────

func TestPosterOverlay_NoPosterURL_NoCache(t *testing.T) {
	svc, deps, mockMgr, item := setupPosterOverlayTestFull(t)

	// Delete the pre-seeded cache to force a download attempt
	tmdbID := *item.TmdbID
	cacheKey := poster.CacheKey(0, tmdbID, "canonical")
	_ = svc.cache.Delete(cacheKey)

	// Set empty PosterURL — service should return nil without upload
	item.PosterURL = ""

	err := svc.UpdateOverlay(item, 7, "countdown", deps)
	if err != nil {
		t.Fatalf("UpdateOverlay should not error with empty PosterURL: %v", err)
	}

	// No upload should have occurred
	if _, ok := mockMgr.getUploaded("plex-12345"); ok {
		t.Error("Expected no poster upload when PosterURL is empty and cache is missing")
	}
}

// ─── ValidateCache with active overlay and missing cache ────────────────────

func TestPosterOverlay_ValidateCache_MissingEntry(t *testing.T) {
	database := setupTestDB(t)
	bus := events.NewEventBus()
	t.Cleanup(func() { bus.Close() })

	cacheDir := t.TempDir()
	svc, err := NewPosterOverlayService(database, bus, cacheDir)
	if err != nil {
		t.Fatalf("NewPosterOverlayService failed: %v", err)
	}

	// Seed FK prerequisites
	integrationID := seedIntegration(t, database)
	diskGroupID := seedDiskGroup(t, database)

	// Insert a sunset item with poster_overlay_active=true
	tmdbID := 13029
	item := db.SunsetQueueItem{
		MediaName:           "Firefly",
		MediaType:           "show",
		TmdbID:              &tmdbID,
		IntegrationID:       integrationID,
		DiskGroupID:         diskGroupID,
		Status:              db.SunsetStatusPending,
		PosterOverlayActive: true,
	}
	if err := database.Create(&item).Error; err != nil {
		t.Fatalf("Failed to create sunset item: %v", err)
	}

	// Do NOT seed the cache — ValidateCache should detect the missing entry
	// and not panic. We can't easily assert the log output, but we verify
	// it completes without error.
	svc.ValidateCache()
}

// ─── ValidateCache with active overlay and present cache ────────────────────

func TestPosterOverlay_ValidateCache_CachePresent(t *testing.T) {
	database := setupTestDB(t)
	bus := events.NewEventBus()
	t.Cleanup(func() { bus.Close() })

	cacheDir := t.TempDir()
	svc, err := NewPosterOverlayService(database, bus, cacheDir)
	if err != nil {
		t.Fatalf("NewPosterOverlayService failed: %v", err)
	}

	// Seed FK prerequisites
	integrationID := seedIntegration(t, database)
	diskGroupID := seedDiskGroup(t, database)

	tmdbID := 13029
	item := db.SunsetQueueItem{
		MediaName:           "Firefly",
		MediaType:           "show",
		TmdbID:              &tmdbID,
		IntegrationID:       integrationID,
		DiskGroupID:         diskGroupID,
		Status:              db.SunsetStatusPending,
		PosterOverlayActive: true,
	}
	if err := database.Create(&item).Error; err != nil {
		t.Fatalf("Failed to create sunset item: %v", err)
	}

	// Seed cache with a key that contains the TMDb ID pattern
	cacheKey := poster.CacheKey(0, tmdbID, "canonical")
	if err := svc.cache.Store(cacheKey, createTestPosterJPEG(100, 150)); err != nil {
		t.Fatalf("Failed to seed cache: %v", err)
	}

	// Should complete without issues — cache is present
	svc.ValidateCache()
}

// ─── UpdateAll with mixed statuses ──────────────────────────────────────────

func TestPosterOverlay_UpdateAll_Empty(t *testing.T) {
	svc, bus := setupPosterOverlayTest(t)
	sunset := NewSunsetService(setupTestDB(t), bus)

	updated, err := svc.UpdateAll(sunset, "countdown", PosterDeps{})
	if err != nil {
		t.Fatalf("UpdateAll failed: %v", err)
	}
	if updated != 0 {
		t.Errorf("Expected 0 updated for empty queue, got %d", updated)
	}
}
