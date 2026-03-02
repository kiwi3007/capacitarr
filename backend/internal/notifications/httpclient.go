package notifications

import (
	"net/http"
	"time"
)

// webhookHTTPClient is a shared HTTP client for outbound webhook requests.
// 10-second timeout keeps notification sends from blocking too long.
var webhookHTTPClient = &http.Client{
	Timeout: 10 * time.Second,
}
