package routes

// BuildCSP returns the full Content-Security-Policy header value for the
// given nonce. This is the single source of truth for CSP policy — used by
// both the production middleware (main.go) and the test middleware (testutil).
//
// Policy directives:
//   - script-src: nonce-based — allows the server's own inline scripts
//     (theme/splash loader, Nuxt runtime config) while blocking injected scripts.
//   - style-src: 'unsafe-inline' is required by Vue/Nuxt runtime styles.
//   - img-src: data: URIs (inline SVGs, base64 favicons), https: (poster images
//     from TMDB/TVDB CDNs), http: (poster images proxied through local *arr integrations).
//   - connect-src: 'self' covers API calls and SSE.
func BuildCSP(nonce string) string {
	return "default-src 'self'; " +
		"script-src 'self' 'nonce-" + nonce + "'; " +
		"style-src 'self' 'unsafe-inline'; " +
		"img-src 'self' data: https: http:; " +
		"font-src 'self'; " +
		"connect-src 'self'; " +
		"frame-ancestors 'none'; " +
		"base-uri 'self'; " +
		"form-action 'self'"
}
