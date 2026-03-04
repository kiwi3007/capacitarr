package routes

// Integration type constants used across route handlers.
const (
	intTypeSonarr    = "sonarr"
	intTypeRadarr    = "radarr"
	intTypeLidarr    = "lidarr"
	intTypeReadarr   = "readarr"
	intTypePlex      = "plex"
	intTypeTautulli  = "tautulli"
	intTypeOverseerr = "overseerr"
	intTypeJellyfin  = "jellyfin"
	intTypeEmby      = "emby"
)

// URL scheme constants for webhook/URL validation.
const (
	schemeHTTP  = "http"
	schemeHTTPS = "https"
)
