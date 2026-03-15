package db

import "testing"

func TestDiskGroup_EffectiveTotalBytes(t *testing.T) {
	tests := []struct {
		name     string
		group    DiskGroup
		expected int64
	}{
		{
			name:     "nil override returns detected total",
			group:    DiskGroup{TotalBytes: 1000000000, TotalBytesOverride: nil},
			expected: 1000000000,
		},
		{
			name:     "zero override returns detected total",
			group:    DiskGroup{TotalBytes: 1000000000, TotalBytesOverride: int64Ptr(0)},
			expected: 1000000000,
		},
		{
			name:     "negative override returns detected total",
			group:    DiskGroup{TotalBytes: 1000000000, TotalBytesOverride: int64Ptr(-500)},
			expected: 1000000000,
		},
		{
			name:     "positive override returns override value",
			group:    DiskGroup{TotalBytes: 1000000000, TotalBytesOverride: int64Ptr(500000000)},
			expected: 500000000,
		},
		{
			name:     "override larger than detected returns override",
			group:    DiskGroup{TotalBytes: 1000000000, TotalBytesOverride: int64Ptr(2000000000)},
			expected: 2000000000,
		},
		{
			name:     "override with zero detected total returns override",
			group:    DiskGroup{TotalBytes: 0, TotalBytesOverride: int64Ptr(1099511627776)},
			expected: 1099511627776,
		},
		{
			name:     "both zero returns zero",
			group:    DiskGroup{TotalBytes: 0, TotalBytesOverride: nil},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.group.EffectiveTotalBytes()
			if result != tt.expected {
				t.Errorf("EffectiveTotalBytes() = %d, want %d", result, tt.expected)
			}
		})
	}
}

func int64Ptr(v int64) *int64 {
	return &v
}
