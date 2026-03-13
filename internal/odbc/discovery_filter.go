package odbc

import "strings"

func FilterDSNs(input []DSNEntry, term string, includeUser, includeSystem bool) []DSNEntry {
	term = strings.ToLower(strings.TrimSpace(term))
	result := make([]DSNEntry, 0, len(input))
	for _, item := range input {
		if item.Scope == "user" && !includeUser {
			continue
		}
		if item.Scope == "system" && !includeSystem {
			continue
		}
		if term == "" {
			result = append(result, item)
			continue
		}
		if strings.Contains(strings.ToLower(item.Name), term) || strings.Contains(strings.ToLower(item.Driver), term) {
			result = append(result, item)
		}
	}
	return result
}

func FilterDrivers(input []DriverEntry, term string) []DriverEntry {
	term = strings.ToLower(strings.TrimSpace(term))
	if term == "" {
		return input
	}
	result := make([]DriverEntry, 0, len(input))
	for _, item := range input {
		if strings.Contains(strings.ToLower(item.Name), term) {
			result = append(result, item)
		}
	}
	return result
}
