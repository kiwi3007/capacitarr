package services

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"

	"capacitarr/internal/db"
	"capacitarr/internal/events"
)

// BackupInfo describes a single automatic database backup file.
type BackupInfo struct {
	Path      string    `json:"path"`
	Date      time.Time `json:"date"`
	SizeBytes int64     `json:"sizeBytes"`
}

// SettingsProvider reads preferences for the backup service. Implemented by
// SettingsService; defined as an interface to avoid a circular dependency
// during construction.
type SettingsProvider interface {
	GetPreferences() (db.PreferenceSet, error)
}

// DatabaseBackupService performs scheduled VACUUM INTO backups of the SQLite
// database and rotates old backup files based on the user's retention setting.
type DatabaseBackupService struct {
	database  *gorm.DB
	bus       *events.EventBus
	dbPath    string
	backupDir string
	settings  SettingsProvider
}

// NewDatabaseBackupService creates a new DatabaseBackupService. The backupDir
// is derived from the database file path: /config/capacitarr.db → /config/backups/.
func NewDatabaseBackupService(database *gorm.DB, bus *events.EventBus, dbPath string) *DatabaseBackupService {
	return &DatabaseBackupService{
		database:  database,
		bus:       bus,
		dbPath:    dbPath,
		backupDir: filepath.Join(filepath.Dir(dbPath), "backups"),
	}
}

// SetSettingsService wires the lazy dependency on SettingsService.
func (s *DatabaseBackupService) SetSettingsService(settings SettingsProvider) {
	s.settings = settings
}

// Wired returns true when all lazy dependencies have been injected.
func (s *DatabaseBackupService) Wired() bool {
	return s.settings != nil
}

// backupFilePrefix is the filename prefix for automatic backup files.
const backupFilePrefix = "capacitarr-"

// backupFileSuffix is the filename suffix for automatic backup files.
const backupFileSuffix = ".db"

// backupDateFormat is the date format embedded in backup filenames.
const backupDateFormat = "20060102"

// RunScheduledBackup creates a backup of the live database via VACUUM INTO,
// then rotates old backups according to the retention setting. This is the
// main entry point called by the daily cron job and on first startup.
func (s *DatabaseBackupService) RunScheduledBackup() error {
	// Ensure the backup directory exists
	if err := os.MkdirAll(s.backupDir, 0o750); err != nil {
		s.bus.Publish(events.DatabaseBackupFailedEvent{Error: err.Error()})
		return fmt.Errorf("failed to create backup directory %s: %w", s.backupDir, err)
	}

	// Build the backup filename for today
	today := time.Now().UTC().Format(backupDateFormat)
	destPath := filepath.Join(s.backupDir, backupFilePrefix+today+backupFileSuffix)

	// Execute VACUUM INTO
	if err := s.vacuumInto(destPath); err != nil {
		s.bus.Publish(events.DatabaseBackupFailedEvent{Error: err.Error()})
		return err
	}

	// Get file size for logging/events
	stat, err := os.Stat(destPath)
	if err != nil {
		slog.Warn("Backup created but could not stat file", "component", "database_backup", "path", destPath, "error", err)
	}

	// Rotate old backups
	retentionDays := 7 // default
	if s.settings != nil {
		prefs, prefsErr := s.settings.GetPreferences()
		if prefsErr != nil {
			slog.Error("Failed to read preferences for backup rotation — using default retention",
				"component", "database_backup", "error", prefsErr, "defaultDays", retentionDays)
		} else if prefs.BackupRetentionDays > 0 {
			retentionDays = prefs.BackupRetentionDays
		}
	}

	retained, rotateErr := s.rotate(retentionDays)
	if rotateErr != nil {
		slog.Error("Backup completed but rotation failed", "component", "database_backup", "error", rotateErr)
	}

	var sizeBytes int64
	if stat != nil {
		sizeBytes = stat.Size()
	}

	slog.Info("Database backup completed",
		"component", "database_backup",
		"path", destPath,
		"sizeBytes", sizeBytes,
		"maxBackups", retentionDays,
		"backupsRetained", retained,
	)

	s.bus.Publish(events.DatabaseBackupCompletedEvent{
		Path:            destPath,
		SizeBytes:       sizeBytes,
		BackupsRetained: retained,
	})

	return nil
}

// vacuumInto creates a consistent backup of the live database at destPath
// using SQLite's VACUUM INTO statement. This works safely on an open database
// in WAL mode without requiring locks or closing connections.
func (s *DatabaseBackupService) vacuumInto(destPath string) error {
	if err := s.database.Exec("VACUUM INTO ?", destPath).Error; err != nil {
		return fmt.Errorf("VACUUM INTO failed: %w", err)
	}
	return nil
}

// rotate keeps the most recent retentionDays backup files and deletes the rest.
// The retention setting is count-based, not date-based: "7 days" means "keep 7
// backup files" regardless of how far apart they are in calendar time. This
// prevents data loss when the app is offline for extended periods — existing
// backups are preserved until new ones push them out.
//
// Returns the number of backups retained after rotation.
func (s *DatabaseBackupService) rotate(retentionDays int) (int, error) {
	backups, err := s.ListBackups()
	if err != nil {
		return 0, fmt.Errorf("failed to list backups for rotation: %w", err)
	}

	// ListBackups returns oldest-first; keep the last retentionDays entries
	if len(backups) <= retentionDays {
		return len(backups), nil
	}

	toDelete := backups[:len(backups)-retentionDays]
	var errs []string

	for _, b := range toDelete {
		if removeErr := os.Remove(b.Path); removeErr != nil {
			errs = append(errs, fmt.Sprintf("failed to remove %s: %v", b.Path, removeErr))
		} else {
			slog.Debug("Removed old backup", "component", "database_backup", "path", b.Path, "date", b.Date.Format(backupDateFormat))
		}
	}

	retained := len(backups) - len(toDelete)
	if len(errs) > 0 {
		return retained, fmt.Errorf("rotation errors: %s", strings.Join(errs, "; "))
	}
	return retained, nil
}

// ListBackups returns all automatic backup files sorted by date (oldest first).
func (s *DatabaseBackupService) ListBackups() ([]BackupInfo, error) {
	entries, err := os.ReadDir(s.backupDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read backup directory: %w", err)
	}

	var backups []BackupInfo
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, backupFilePrefix) || !strings.HasSuffix(name, backupFileSuffix) {
			continue
		}

		// Parse date from filename: capacitarr-YYYYMMDD.db
		dateStr := strings.TrimPrefix(name, backupFilePrefix)
		dateStr = strings.TrimSuffix(dateStr, backupFileSuffix)
		date, parseErr := time.Parse(backupDateFormat, dateStr)
		if parseErr != nil {
			continue // skip files with unparseable dates
		}

		info, statErr := entry.Info()
		if statErr != nil {
			continue
		}

		backups = append(backups, BackupInfo{
			Path:      filepath.Join(s.backupDir, name),
			Date:      date,
			SizeBytes: info.Size(),
		})
	}

	sort.Slice(backups, func(i, j int) bool {
		return backups[i].Date.Before(backups[j].Date)
	})

	return backups, nil
}

// LatestBackupPath returns the path to the most recent automatic backup,
// or an empty string if no backups exist. Used by error messages to direct
// the user to a recovery file.
func (s *DatabaseBackupService) LatestBackupPath() (string, error) {
	backups, err := s.ListBackups()
	if err != nil {
		return "", err
	}
	if len(backups) == 0 {
		return "", nil
	}
	return backups[len(backups)-1].Path, nil
}
