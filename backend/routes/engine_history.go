package routes

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"capacitarr/internal/db"
)

// RegisterEngineHistoryRoutes registers engine history endpoints on the protected group.
func RegisterEngineHistoryRoutes(g *echo.Group, database *gorm.DB) {
	g.GET("/engine/history", handleEngineHistory(database))
}

func handleEngineHistory(database *gorm.DB) echo.HandlerFunc {
	return func(c echo.Context) error {
		rangeParam := c.QueryParam("range")
		if rangeParam == "" {
			rangeParam = "7d"
		}

		dur, err := parseDuration(rangeParam)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid range parameter"})
		}

		cutoff := time.Now().UTC().Add(-dur)

		var stats []db.EngineRunStats
		if err := database.Where("run_at >= ?", cutoff).
			Order("run_at ASC").
			Find(&stats).Error; err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to query engine history"})
		}

		// Build response with 4 series for sparklines
		type HistoryPoint struct {
			Timestamp  time.Time `json:"timestamp"`
			Evaluated  int       `json:"evaluated"`
			Flagged    int       `json:"flagged"`
			Deleted    int       `json:"deleted"`
			FreedBytes int64     `json:"freedBytes"`
			DurationMs int64     `json:"durationMs"`
		}

		points := make([]HistoryPoint, len(stats))
		for i, s := range stats {
			points[i] = HistoryPoint{
				Timestamp:  s.RunAt,
				Evaluated:  s.Evaluated,
				Flagged:    s.Flagged,
				Deleted:    s.Deleted,
				FreedBytes: s.FreedBytes,
				DurationMs: s.DurationMs,
			}
		}

		return c.JSON(http.StatusOK, points)
	}
}
