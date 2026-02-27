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
			var tokenStr string

			// Check Authorization header first
			authHeader := c.Request().Header.Get("Authorization")
			if authHeader != "" {
				parts := strings.Split(authHeader, " ")
				if len(parts) != 2 {
					return echo.ErrUnauthorized
				}

				if parts[0] == "Bearer" {
					tokenStr = parts[1]
				} else if parts[0] == "ApiKey" {
					apiKey := parts[1]
					var auth db.AuthConfig
					if err := database.Where("api_key = ?", apiKey).First(&auth).Error; err != nil {
						return echo.ErrUnauthorized
					}
					c.Set("user", auth.Username)
					return next(c)
				} else {
					return echo.ErrUnauthorized
				}
			}

			// Fallback: check jwt cookie
			if tokenStr == "" {
				cookie, err := c.Cookie("jwt")
				if err != nil || cookie.Value == "" {
					return echo.ErrUnauthorized
				}
				tokenStr = cookie.Value
			}

			// Validate JWT token
			token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
				return []byte(cfg.JWTSecret), nil
			})

			if err != nil || !token.Valid {
				return echo.ErrUnauthorized
			}

			claims := token.Claims.(jwt.MapClaims)
			c.Set("user", claims["sub"].(string))
			return next(c)
		}
	}
}
