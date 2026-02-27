package db

import (
	"time"
)

// AuthConfig stores credentials for web UI sessions
type AuthConfig struct {
	ID        uint      `gorm:"primarykey"`
	Username  string    `gorm:"uniqueIndex;not null"`
	Password  string    `gorm:"not null"` // Hashed password
	APIKey    string    `gorm:"index"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

// LibraryHistory stores historical capacity logs
type LibraryHistory struct {
	ID            uint      `gorm:"primarykey"`
	Timestamp     time.Time `gorm:"index;not null"`
	TotalCapacity int64     `gorm:"not null"`
	UsedCapacity  int64     `gorm:"not null"`
	Resolution    string    `gorm:"index;not null"` // "raw", "hourly", "daily", "weekly"
	DiskGroupID   *uint     `gorm:"index"`          // Optional FK to DiskGroup
	CreatedAt     time.Time
}

// IntegrationConfig stores a configured service connection
type IntegrationConfig struct {
	ID             uint       `gorm:"primarykey" json:"id"`
	Type           string     `gorm:"not null;index" json:"type"`   // plex, sonarr, radarr
	Name           string     `gorm:"not null" json:"name"`         // User-friendly name
	URL            string     `gorm:"not null" json:"url"`
	APIKey         string     `gorm:"not null" json:"apiKey"`       // API key or Plex token
	Enabled        bool       `gorm:"default:true" json:"enabled"`
	MediaSizeBytes int64      `json:"mediaSizeBytes"`               // Total media file size
	MediaCount     int        `json:"mediaCount"`                   // Number of media items
	LastSync       *time.Time `json:"lastSync,omitempty"`
	LastError      string     `json:"lastError,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}

// DiskGroup represents a physical disk/mount point shared by multiple services
type DiskGroup struct {
	ID           uint    `gorm:"primarykey" json:"id"`
	MountPath    string  `gorm:"uniqueIndex;not null" json:"mountPath"`
	TotalBytes   int64   `gorm:"not null" json:"totalBytes"`
	UsedBytes    int64   `gorm:"not null" json:"usedBytes"`
	ThresholdPct float64 `gorm:"default:85" json:"thresholdPct"` // Clean up at this %
	TargetPct    float64 `gorm:"default:75" json:"targetPct"`    // Free down to this %
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}
