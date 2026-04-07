package http

import (
	"math/rand"
	"os"
	"strings"
	"time"
)

var userAgents = [...]string{
	"Mozilla/5.0 (X11; Linux x86_64; rv:140.0) Gecko/20100101 Firefox/140.0",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:138.0) Gecko/20100101 Firefox/138.0",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 14.7; rv:139.0) Gecko/20100101 Firefox/139.0",
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/132.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36",
}

// RandomUA returns a random user agent from the built-in pool.
func RandomUA() string {
	return userAgents[rand.Intn(len(userAgents))]
}

// SystemTimezone returns the system's IANA timezone name.
// Falls back to "UTC" if detection fails or returns "Local".
func SystemTimezone() string {
	// Try TZ env var first (most explicit)
	if tz := tzFromEnv(); tz != "" {
		return tz
	}
	// Try /etc/timezone (Debian/Ubuntu)
	if tz := tzFromEtcTimezone(); tz != "" {
		return tz
	}
	// Try /etc/localtime symlink (RHEL/Arch/macOS)
	if tz := tzFromLocaltimeLink(); tz != "" {
		return tz
	}
	// Go stdlib fallback
	loc := time.Now().Location().String()
	if loc == "" || loc == "Local" {
		return "UTC"
	}
	return loc
}

func tzFromEnv() string {
	tz := strings.TrimSpace(os.Getenv("TZ"))
	if tz != "" && strings.Contains(tz, "/") {
		return tz
	}
	return ""
}

func tzFromEtcTimezone() string {
	data, err := os.ReadFile("/etc/timezone")
	if err != nil {
		return ""
	}
	tz := strings.TrimSpace(string(data))
	if tz != "" && strings.Contains(tz, "/") {
		return tz
	}
	return ""
}

func tzFromLocaltimeLink() string {
	target, err := os.Readlink("/etc/localtime")
	if err != nil {
		return ""
	}
	parts := strings.SplitN(target, "/zoneinfo/", 2)
	if len(parts) == 2 && parts[1] != "" {
		return parts[1]
	}
	return ""
}

// SystemLocale returns (language, region) detected from the system.
// Tries LC_ALL, then LANG env var. Falls back to ("en", "US").
func SystemLocale() (string, string) {
	for _, v := range []string{"LC_ALL", "LANG"} {
		val := strings.TrimSpace(os.Getenv(v))
		if val == "" || val == "C" || val == "POSIX" {
			continue
		}
		lang, region := parsePosixLocale(val)
		if lang != "" {
			return lang, region
		}
	}
	return "en", "US"
}

// SystemLanguage returns the detected language code (e.g. "en", "ro").
func SystemLanguage() string {
	lang, _ := SystemLocale()
	return lang
}

// SystemRegion returns the detected region code (e.g. "US", "RO").
func SystemRegion() string {
	_, region := SystemLocale()
	return region
}

func parsePosixLocale(s string) (string, string) {
	// Strip encoding: "en_US.UTF-8" -> "en_US"
	if idx := strings.IndexByte(s, '.'); idx >= 0 {
		s = s[:idx]
	}
	// Split on _ or -
	s = strings.ReplaceAll(s, "-", "_")
	parts := strings.SplitN(s, "_", 2)
	lang := strings.ToLower(parts[0])
	if len(lang) < 2 {
		return "", ""
	}
	region := "US"
	if len(parts) > 1 {
		region = strings.ToUpper(parts[1])
	}
	return lang, region
}
