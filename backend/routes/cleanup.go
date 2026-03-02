package routes

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// RegisterCleanupRoutes sets up the API endpoints for cleanup history sparklines.
func RegisterCleanupRoutes(g *echo.Group, database *gorm.DB) {
	g.GET("/cleanup-history", handleCleanupHistory(database))
}

// CleanupPoint represents a single time-bucketed cleanup event for sparkline charts.
type CleanupPoint struct {
	Timestamp      string `json:"timestamp"`
	ItemsDeleted   int    `json:"itemsDeleted"`
	BytesReclaimed int64  `json:"bytesReclaimed"`
}

func handleCleanupHistory(database *gorm.DB) echo.HandlerFunc {
	return func(c echo.Context) error {
		rangeParam := c.QueryParam("range")
		if rangeParam == "" {
			rangeParam = "24h"
		}

		dur, err := parseDuration(rangeParam)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": "invalid range parameter; supported values: 24h, 7d, 30d, 90d",
			})
		}

		cutoff := time.Now().UTC().Add(-dur).Format("2006-01-02 15:04:05")
		bucketExpr := cleanupBucketExpr(dur)

		var rows []CleanupPoint
		query := `SELECT ` + bucketExpr + ` AS timestamp,
			COALESCE(SUM(flagged), 0) AS items_deleted,
			COALESCE(SUM(freed_bytes), 0) AS bytes_reclaimed
			FROM engine_run_stats WHERE run_at >= ?
			GROUP BY timestamp ORDER BY timestamp`

		if err := database.Raw(query, cutoff).Scan(&rows).Error; err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "failed to query cleanup history",
			})
		}

		// Return empty array instead of null when no data
		if rows == nil {
			rows = []CleanupPoint{}
		}

		return c.JSON(http.StatusOK, rows)
	}
}

// cleanupBucketExpr returns a SQLite expression that buckets run_at timestamps
// into ISO 8601 strings at the appropriate granularity for the given range.
func cleanupBucketExpr(d time.Duration) string {
	switch {
	case d <= 24*time.Hour:
		// Hourly buckets
		return "strftime('%Y-%m-%dT%H:00:00Z', run_at)"
	case d <= 7*24*time.Hour:
		// 6-hour buckets
		return "strftime('%Y-%m-%dT', run_at) || printf('%02d', (CAST(strftime('%H', run_at) AS INTEGER) / 6) * 6) || ':00:00Z'"
	default:
		// Daily buckets (30d, 90d)
		return "strftime('%Y-%m-%dT00:00:00Z', run_at)"
	}
}
