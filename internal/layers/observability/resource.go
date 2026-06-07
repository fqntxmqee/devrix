package observability

import (
	"os"
	"runtime/debug"
	"strings"
)

// Resource represents the service resource
type Resource struct {
	ServiceName    string
	ServiceVersion string
	Environment   string
	GitCommit     string
	BuildTime     string
}

// DefaultResource creates a resource with build info
func DefaultResource() *Resource {
	r := &Resource{
		ServiceName:    "devrix",
		ServiceVersion: "1.0.0",
		Environment:   getEnv("DEVRIX_ENV", "development"),
	}

	// Try to get build info
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, s := range info.Settings {
			if s.Key == "vcs.revision" {
				r.GitCommit = truncate(s.Value, 12)
			}
			if s.Key == "vcs.time" {
				r.BuildTime = s.Value
			}
		}
	}

	return r
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); strings.TrimSpace(val) != "" {
		return val
	}
	return defaultVal
}

// Attributes returns resource as a map of attributes
func (r *Resource) Attributes() map[string]string {
	return map[string]string{
		"service.name":             r.ServiceName,
		"service.version":          r.ServiceVersion,
		"deployment.environment":   r.Environment,
		"service.namespace":        "devrix",
	}
}
