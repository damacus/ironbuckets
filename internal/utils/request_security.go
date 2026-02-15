package utils

import (
	"net/http"
	"strings"
)

func IsSecureRequest(req *http.Request) bool {
	if req == nil {
		return false
	}

	if req.TLS != nil {
		return true
	}

	return strings.EqualFold(req.Header.Get("X-Forwarded-Proto"), "https")
}
