package main

import (
	"net/http"
	"slices"
	"testing"
)

func TestBuildCORSConfigAllowsPatchRequests(t *testing.T) {
	config := buildCORSConfig()

	if !slices.Contains(config.AllowMethods, http.MethodPatch) {
		t.Fatalf("expected CORS config to allow %s, got %v", http.MethodPatch, config.AllowMethods)
	}
}
