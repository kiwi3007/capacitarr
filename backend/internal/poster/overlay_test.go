package poster

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"testing"
)

// createTestPoster generates a minimal JPEG poster for testing.
func createTestPoster(w, h int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	// Fill with a dark blue (movie poster-like)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: 20, G: 30, B: 80, A: 255})
		}
	}
	var buf bytes.Buffer
	_ = jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85})
	return buf.Bytes()
}

func TestComposeOverlay_30Days(t *testing.T) {
	poster := createTestPoster(300, 450)
	result, err := ComposeOverlay(poster, 30, "countdown")
	if err != nil {
		t.Fatalf("ComposeOverlay failed: %v", err)
	}
	if len(result) == 0 {
		t.Fatal("Expected non-empty result")
	}
	// Verify it's valid JPEG
	_, err = jpeg.Decode(bytes.NewReader(result))
	if err != nil {
		t.Fatalf("Result is not valid JPEG: %v", err)
	}
}

func TestComposeOverlay_1Day(t *testing.T) {
	poster := createTestPoster(300, 450)
	result, err := ComposeOverlay(poster, 1, "countdown")
	if err != nil {
		t.Fatalf("ComposeOverlay failed: %v", err)
	}
	if len(result) == 0 {
		t.Fatal("Expected non-empty result")
	}
}

func TestComposeOverlay_LastDay(t *testing.T) {
	poster := createTestPoster(300, 450)
	result, err := ComposeOverlay(poster, 0, "countdown")
	if err != nil {
		t.Fatalf("ComposeOverlay failed: %v", err)
	}
	if len(result) == 0 {
		t.Fatal("Expected non-empty result")
	}
}

func TestComposeOverlay_SimpleStyle(t *testing.T) {
	poster := createTestPoster(300, 450)
	result, err := ComposeOverlay(poster, 30, "simple")
	if err != nil {
		t.Fatalf("ComposeOverlay with simple style failed: %v", err)
	}
	if len(result) == 0 {
		t.Fatal("Expected non-empty result")
	}
	// Verify it's valid JPEG
	_, err = jpeg.Decode(bytes.NewReader(result))
	if err != nil {
		t.Fatalf("Result is not valid JPEG: %v", err)
	}
}

func TestComposeOverlay_CorruptInput(t *testing.T) {
	_, err := ComposeOverlay([]byte("not an image"), 30, "countdown")
	if err == nil {
		t.Fatal("Expected error for corrupt input")
	}
}

func TestComposeOverlay_TinyImage(t *testing.T) {
	poster := createTestPoster(5, 5)
	_, err := ComposeOverlay(poster, 30, "countdown")
	if err == nil {
		t.Fatal("Expected error for tiny image")
	}
}

func TestContentHash(t *testing.T) {
	data := []byte("test image data")
	hash1 := ContentHash(data)
	hash2 := ContentHash(data)
	if hash1 != hash2 {
		t.Errorf("Expected consistent hash, got %s and %s", hash1, hash2)
	}
	if len(hash1) != 32 {
		t.Errorf("Expected 32 char hex hash, got %d chars", len(hash1))
	}

	differentHash := ContentHash([]byte("different data"))
	if hash1 == differentHash {
		t.Error("Expected different hashes for different data")
	}
}

func TestCountdownText(t *testing.T) {
	tests := []struct {
		days     int
		style    string
		expected string
	}{
		{0, "countdown", "Last day"},
		{-1, "countdown", "Last day"},
		{1, "countdown", "Leaving tomorrow"},
		{7, "countdown", "Leaving in 7 days"},
		{30, "countdown", "Leaving in 30 days"},
		{0, "simple", "Leaving soon"},
		{1, "simple", "Leaving soon"},
		{7, "simple", "Leaving soon"},
		{30, "simple", "Leaving soon"},
		{-1, "simple", "Leaving soon"},
	}
	for _, tt := range tests {
		got := countdownText(tt.days, tt.style)
		if got != tt.expected {
			t.Errorf("countdownText(%d, %q) = %q, want %q", tt.days, tt.style, got, tt.expected)
		}
	}
}
