package odbc

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	sqlStatePattern = regexp.MustCompile(`\b([A-Z0-9]{5})\b`)
	nativePattern   = regexp.MustCompile(`\((-?\d+)\)`)
)

func BuildDiagnostic(operation, sqlText string, err error) Diagnostic {
	diag := Diagnostic{
		Operation: operation,
		At:        time.Now(),
		SQL:       MaskSecrets(sqlText),
	}
	if err == nil {
		return diag
	}
	msg := err.Error()
	diag.Message = MaskSecrets(msg)
	lines := strings.Split(msg, "\n")
	records := make([]DiagRecord, 0, len(lines))
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		record := DiagRecord{Message: MaskSecrets(line)}
		if state := sqlStatePattern.FindString(line); state != "" {
			record.State = state
		}
		if native := nativePattern.FindStringSubmatch(line); len(native) > 1 {
			if n, convErr := strconv.Atoi(native[1]); convErr == nil {
				record.NativeError = n
			}
		}
		records = append(records, record)
	}
	if len(records) == 0 {
		records = append(records, DiagRecord{Message: diag.Message})
	}
	diag.Records = records
	return diag
}
