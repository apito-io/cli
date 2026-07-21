package main

import "strings"

// AccessTokenPrefix matches the engine's unified access token format
// (see apito/pkg/open-core models.AccessTokenPrefix).
const AccessTokenPrefix = "apt_"

// retiredTokenPrefixes are the legacy per-surface sync-key formats the engine
// now rejects with TOKEN_FORMAT_RETIRED — replaced by unified apt_ access tokens.
var retiredTokenPrefixes = []string{"cli-", "sdk-", "mcp-"}

// TokenFormatRetiredMessage mirrors the engine's rejection message so CLI users
// see the same guidance whether the client or the server catches the old format.
const TokenFormatRetiredMessage = "TOKEN_FORMAT_RETIRED: use apt_ access tokens — generate one in Console → Access Token and run: apito config set account <name> key <apt_...>"

// IsAccessToken reports whether raw looks like a unified apt_ access token.
func IsAccessToken(raw string) bool {
	return strings.HasPrefix(strings.TrimSpace(raw), AccessTokenPrefix)
}

// IsRetiredTokenPrefix reports legacy cli-/sdk-/mcp- token formats.
func IsRetiredTokenPrefix(raw string) bool {
	r := strings.TrimSpace(raw)
	for _, p := range retiredTokenPrefixes {
		if strings.HasPrefix(r, p) {
			return true
		}
	}
	return false
}
