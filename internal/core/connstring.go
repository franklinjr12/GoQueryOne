package core

import (
	"errors"
	"fmt"
	"strings"

	"github.com/franklinjr12/GoQueryOne/internal/config"
)

// BuildConnectionString assembles an ODBC connection string for profile,
// brace-escaping values per the ODBC spec: a value containing ';', '{',
// '}' or '=', or with leading/trailing spaces, is wrapped in "{...}" with
// any '}' inside doubled to "}}". The driver name is always wrapped, since
// driver display names routinely contain spaces and commas.
func BuildConnectionString(profile config.ConnectionProfile, password string) (string, error) {
	base := strings.TrimSpace(profile.ConnectionString)
	switch strings.ToLower(strings.TrimSpace(profile.Type)) {
	case "dsn":
		if strings.TrimSpace(profile.DSN) == "" {
			return "", errors.New("dsn is required")
		}
		base = "DSN=" + escapeConnValue(profile.DSN) + ";"
	case "connection_string":
		if base == "" {
			return "", errors.New("connection string is required")
		}
	case "file_dsn":
		if strings.TrimSpace(profile.FilePath) == "" {
			return "", errors.New("file dsn path is required")
		}
		base = "FILEDSN=" + escapeConnValue(profile.FilePath) + ";"
	case "driver":
		if strings.TrimSpace(profile.Driver) == "" {
			return "", errors.New("driver is required")
		}
		// In Driver mode ConnectionString holds the extra attributes
		// (Server=...;Database=...;) appended after the driver.
		extra := strings.TrimSpace(base)
		base = "Driver=" + braceConnValue(profile.Driver) + ";"
		if extra != "" {
			if !strings.HasSuffix(extra, ";") {
				extra += ";"
			}
			base += strings.TrimLeft(extra, "; ")
		}
	default:
		return "", fmt.Errorf("unsupported connection profile type %q", profile.Type)
	}
	if profile.Username != "" && !containsConnKey(base, "uid") && !containsConnKey(base, "user id") {
		base += "UID=" + escapeConnValue(profile.Username) + ";"
	}
	if password != "" && !containsConnKey(base, "pwd") && !containsConnKey(base, "password") {
		base += "PWD=" + escapeConnValue(password) + ";"
	}
	return base, nil
}

func needsConnBrace(v string) bool {
	if v == "" {
		return false
	}
	if strings.HasPrefix(v, " ") || strings.HasSuffix(v, " ") {
		return true
	}
	return strings.ContainsAny(v, ";{}=")
}

func braceConnValue(v string) string {
	return "{" + strings.ReplaceAll(v, "}", "}}") + "}"
}

func escapeConnValue(v string) string {
	if needsConnBrace(v) {
		return braceConnValue(v)
	}
	return v
}

func containsConnKey(connStr, key string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(connStr, " ", ""))
	key = strings.ToLower(strings.ReplaceAll(key, " ", ""))
	return strings.Contains(normalized, key+"=")
}
