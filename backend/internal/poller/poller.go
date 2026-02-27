package poller

import (
	"log/slog"
	"strings"
	"time"

	"capacitarr/internal/db"
	"capacitarr/internal/integrations"
)

// Start begins the continuous polling loop.
// It queries all enabled integrations, fetches disk space for media root folders only,
// updates DiskGroups, and records a LibraryHistory snapshot per disk group.
func Start(interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			poll()
		}
	}()
}

func poll() {
	var configs []db.IntegrationConfig
	if err := db.DB.Where("enabled = ?", true).Find(&configs).Error; err != nil {
		slog.Error("Poller: failed to load integrations", "error", err)
		return
	}

	if len(configs) == 0 {
		return
	}

	// Collect root folder paths and disk space from *arr integrations
	rootFolders := make(map[string]bool)       // set of root folder paths
	diskMap := make(map[string]integrations.DiskSpace) // all disk entries from *arr

	for _, cfg := range configs {
		client := createClient(cfg.Type, cfg.URL, cfg.APIKey)
		if client == nil {
			continue
		}

		if cfg.Type == "plex" {
			// Plex is only used for protection rules, not disk usage tracking
			now := time.Now()
			db.DB.Model(&cfg).Updates(map[string]interface{}{
				"last_sync":  &now,
				"last_error": "",
			})
			continue
		}

		// Fetch media items for per-integration usage tracking (Sonarr/Radarr only)
		items, err := client.GetMediaItems()
		if err != nil {
			slog.Warn("Poller: media items fetch failed",
				"integration", cfg.Name, "type", cfg.Type, "error", err)
		} else {
			var totalSize int64
			// For Sonarr, only count show-level items to avoid double-counting seasons
			for _, item := range items {
				if cfg.Type == "sonarr" && item.Type != integrations.MediaTypeShow {
					continue
				}
				totalSize += item.SizeBytes
			}
			mediaCount := len(items)
			if cfg.Type == "sonarr" {
				// Count unique shows only
				mediaCount = 0
				for _, item := range items {
					if item.Type == integrations.MediaTypeShow {
						mediaCount++
					}
				}
			}
			db.DB.Model(&cfg).Updates(map[string]interface{}{
				"media_size_bytes": totalSize,
				"media_count":     mediaCount,
			})
		}

		// Get root folders (Sonarr/Radarr only)
		folders, err := client.GetRootFolders()
		if err != nil {
			slog.Warn("Poller: root folder fetch failed",
				"integration", cfg.Name, "type", cfg.Type, "error", err)
		}
		for _, f := range folders {
			rootFolders[f] = true
			slog.Info("Poller: root folder found",
				"integration", cfg.Name, "path", f)
		}

		// Get disk space
		disks, err := client.GetDiskSpace()
		if err != nil {
			slog.Warn("Poller: disk space fetch failed",
				"integration", cfg.Name, "type", cfg.Type, "error", err)
			db.DB.Model(&cfg).Updates(map[string]interface{}{
				"last_error": err.Error(),
			})
			continue
		}

		// Update last sync time, clear error
		now := time.Now()
		db.DB.Model(&cfg).Updates(map[string]interface{}{
			"last_sync":  &now,
			"last_error": "",
		})

		// Collect all disk entries
		for _, d := range disks {
			if d.Path == "" {
				continue
			}
			if existing, ok := diskMap[d.Path]; ok {
				if d.TotalBytes > existing.TotalBytes {
					diskMap[d.Path] = d
				}
			} else {
				diskMap[d.Path] = d
			}
		}
	}

	// Find the most specific mount for each root folder
	mediaMounts := findMediaMounts(diskMap, rootFolders)

	// Update DiskGroups and record history only for media mounts
	for mountPath := range mediaMounts {
		disk := diskMap[mountPath]
		usedBytes := disk.TotalBytes - disk.FreeBytes

		// Upsert DiskGroup
		var group db.DiskGroup
		result := db.DB.Where("mount_path = ?", mountPath).First(&group)
		if result.Error != nil {
			group = db.DiskGroup{
				MountPath:  mountPath,
				TotalBytes: disk.TotalBytes,
				UsedBytes:  usedBytes,
			}
			db.DB.Create(&group)
		} else {
			db.DB.Model(&group).Updates(map[string]interface{}{
				"total_bytes": disk.TotalBytes,
				"used_bytes":  usedBytes,
			})
		}

		// Record LibraryHistory snapshot
		record := db.LibraryHistory{
			Timestamp:     time.Now(),
			TotalCapacity: disk.TotalBytes,
			UsedCapacity:  usedBytes,
			Resolution:    "raw",
			DiskGroupID:   &group.ID,
		}
		if err := db.DB.Create(&record).Error; err != nil {
			slog.Error("Poller: failed to save capacity record",
				"mount", mountPath, "error", err)
		}
	}

	// Clean up orphaned disk groups that are no longer media mounts
	if len(mediaMounts) > 0 {
		var allGroups []db.DiskGroup
		db.DB.Find(&allGroups)
		for _, g := range allGroups {
			if !mediaMounts[g.MountPath] {
				slog.Info("Poller: removing orphaned disk group",
					"mount", g.MountPath, "id", g.ID)
				db.DB.Where("disk_group_id = ?", g.ID).Delete(&db.LibraryHistory{})
				db.DB.Delete(&g)
			}
		}
	}
}

// findMediaMounts returns only the mount paths that are the most specific match
// for at least one root folder. For example, if mounts are ["/", "/media"] and
// root folder is "/media/movies", only "/media" is returned (not "/").
func findMediaMounts(diskMap map[string]integrations.DiskSpace, rootFolders map[string]bool) map[string]bool {
	mediaMounts := make(map[string]bool)

	for rf := range rootFolders {
		cleanRF := strings.TrimRight(rf, "/")
		bestMount := ""
		bestLen := 0

		for mountPath := range diskMap {
			cleanMount := strings.TrimRight(mountPath, "/")
			// Special case: root "/" matches everything
			if cleanMount == "" {
				if bestLen == 0 {
					bestMount = mountPath
				}
				continue
			}
			// Check if root folder lives under this mount
			if strings.HasPrefix(cleanRF, cleanMount+"/") || cleanRF == cleanMount {
				if len(cleanMount) > bestLen {
					bestLen = len(cleanMount)
					bestMount = mountPath
				}
			}
		}

		if bestMount != "" {
			mediaMounts[bestMount] = true
			slog.Info("Poller: matched root folder to mount",
				"root_folder", rf, "mount", bestMount)
		}
	}

	// If we have both "/" and other more specific mounts, drop "/"
	// This handles Docker/container scenarios where different services
	// see different mount namespaces for the same underlying storage
	if len(mediaMounts) > 1 {
		for m := range mediaMounts {
			if strings.TrimRight(m, "/") == "" {
				slog.Info("Poller: dropping root mount '/' since more specific mounts exist")
				delete(mediaMounts, m)
			}
		}
	}

	return mediaMounts
}

func createClient(intType, url, apiKey string) integrations.Integration {
	switch intType {
	case "sonarr":
		return integrations.NewSonarrClient(url, apiKey)
	case "radarr":
		return integrations.NewRadarrClient(url, apiKey)
	case "plex":
		return integrations.NewPlexClient(url, apiKey)
	default:
		return nil
	}
}
