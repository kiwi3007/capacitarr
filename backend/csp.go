package main

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
)

// cspNoncePlaceholder is the marker injected into HTML templates at startup.
// It is replaced with a fresh cryptographic nonce on every request.
const cspNoncePlaceholder = "__CSP_NONCE__"

// generateCSPNonce produces a cryptographically random, base64url-encoded
// nonce suitable for Content-Security-Policy script-src directives.
// Each call returns a unique 22-character string (16 random bytes).
func generateCSPNonce() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// injectNoncePlaceholders rewrites inline <script> tags in the given HTML
// to include a nonce="__CSP_NONCE__" attribute. Only executable inline
// scripts are modified:
//
//   - <script type="text/javascript">  (theme/splash loader)
//   - <script>window.__NUXT__          (Nuxt runtime config)
//
// External scripts (<script … src="…">) and non-executable scripts
// (<script type="application/json">) are left untouched; they are
// already covered by CSP 'self' or not subject to script-src.
func injectNoncePlaceholders(html []byte) []byte {
	placeholder := []byte(`nonce="` + cspNoncePlaceholder + `"`)

	// Pattern 1: <script type="text/javascript"> → add nonce
	result := bytes.Replace(html,
		[]byte(`<script type="text/javascript">`),
		[]byte(`<script type="text/javascript" `+string(placeholder)+`>`),
		1)

	// Pattern 2: <script>window.__NUXT__ → add nonce
	result = bytes.Replace(result,
		[]byte(`<script>window.__NUXT__`),
		[]byte(`<script `+string(placeholder)+`>window.__NUXT__`),
		1)

	return result
}

// applyNonce replaces all __CSP_NONCE__ placeholders in the HTML template
// with the given nonce value. This is called once per request.
func applyNonce(template []byte, nonce string) []byte {
	return bytes.ReplaceAll(template, []byte(cspNoncePlaceholder), []byte(nonce))
}
