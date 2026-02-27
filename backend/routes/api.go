package routes

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"capacitarr/internal/config"
	"capacitarr/internal/db"
)

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func RegisterAPIRoutes(g *echo.Group, database *gorm.DB, cfg *config.Config) {
	// Health check
	g.GET("/health", func(c echo.Context) error {
		return c.String(http.StatusOK, "OK")
	})

	// Public Auth
	g.POST("/auth/login", func(c echo.Context) error {
		var req LoginRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		}

		var user db.AuthConfig
		if err := database.Where("username = ?", req.Username).First(&user).Error; err != nil {
			// If no user exists in DB at all, bootstrap the first user
			var count int64
			database.Model(&db.AuthConfig{}).Count(&count)
			if count == 0 {
				hashed, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
				user = db.AuthConfig{Username: req.Username, Password: string(hashed)}
				database.Create(&user)
			} else {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid credentials"})
			}
		}

		if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid credentials"})
		}

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"sub": user.Username,
			"exp": time.Now().Add(24 * time.Hour).Unix(),
		})

		tokenString, err := token.SignedString([]byte(cfg.JWTSecret))
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Error generating token"})
		}

		c.SetCookie(&http.Cookie{
			Name:     "jwt",
			Value:    tokenString,
			Expires:  time.Now().Add(24 * time.Hour),
			HttpOnly: false, // SPA needs to read cookie for auth state
			Secure:   false, // Set to true in production
			Path:     "/",
			SameSite: http.SameSiteLaxMode,
		})

		return c.JSON(http.StatusOK, map[string]string{"message": "success", "token": tokenString})
	})

	// Protected Routes
	protected := g.Group("")
	protected.Use(RequireAuth(database, cfg))

	protected.POST("/auth/apikey", func(c echo.Context) error {
		username := c.Get("user").(string)

		bytes := make([]byte, 32)
		if _, err := rand.Read(bytes); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Error generating API key"})
		}
		apiKey := hex.EncodeToString(bytes)

		if err := database.Model(&db.AuthConfig{}).Where("username = ?", username).Update("api_key", apiKey).Error; err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Database error"})
		}

		return c.JSON(http.StatusOK, map[string]string{"api_key": apiKey})
	})

	protected.GET("/metrics/history", func(c echo.Context) error {
		resolution := c.QueryParam("resolution")
		if resolution == "" {
			resolution = "raw"
		}

		diskGroupID := c.QueryParam("disk_group_id")
		since := c.QueryParam("since") // e.g. "1h", "24h", "7d", "30d"

		query := database.Where("resolution = ?", resolution)
		if diskGroupID != "" {
			query = query.Where("disk_group_id = ?", diskGroupID)
		}

		// Apply time range filter
		if since != "" {
			var duration time.Duration
			switch since {
			case "1h":
				duration = 1 * time.Hour
			case "24h":
				duration = 24 * time.Hour
			case "7d":
				duration = 7 * 24 * time.Hour
			case "30d":
				duration = 30 * 24 * time.Hour
			}
			if duration > 0 {
				cutoff := time.Now().Add(-duration)
				query = query.Where("timestamp >= ?", cutoff)
			}
		}

		var history []db.LibraryHistory
		if err := query.Order("timestamp asc").Limit(1000).Find(&history).Error; err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Error fetching metrics"})
		}

		return c.JSON(http.StatusOK, map[string]interface{}{"status": "success", "data": history})
	})

	// Integration management routes
	RegisterIntegrationRoutes(protected, database)
}
