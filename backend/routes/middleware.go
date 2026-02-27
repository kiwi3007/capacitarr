package routes

import (
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"capacitarr/internal/config"
	"capacitarr/internal/db"
)

func RequireAuth(database *gorm.DB, cfg *config.Config) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			authHeader := c.Request().Header.Get("Authorization")
			if authHeader == "" {
				return echo.ErrUnauthorized
			}

			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 {
				return echo.ErrUnauthorized
			}

			if parts[0] == "Bearer" {
				tokenStr := parts[1]
				token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
					return []byte(cfg.JWTSecret), nil
				})

				if err != nil || !token.Valid {
					return echo.ErrUnauthorized
				}

				claims := token.Claims.(jwt.MapClaims)
				c.Set("user", claims["sub"].(string))
				return next(c)
			} else if parts[0] == "ApiKey" {
				apiKey := parts[1]
				var auth db.AuthConfig
				if err := database.Where("api_key = ?", apiKey).First(&auth).Error; err != nil {
					return echo.ErrUnauthorized
				}
				c.Set("user", auth.Username)
				return next(c)
			}

			return echo.ErrUnauthorized
		}
	}
}
