package odbc

import "regexp"

var (
	connPwdPattern = regexp.MustCompile(`(?i)\b(pwd|password)\s*=\s*([^;]+)`)
	jsonPwdPattern = regexp.MustCompile(`(?i)"(password|pwd)"\s*:\s*"([^"]*)"`)
)

func MaskSecrets(value string) string {
	masked := connPwdPattern.ReplaceAllString(value, `${1}=***`)
	masked = jsonPwdPattern.ReplaceAllString(masked, `"$1":"***"`)
	return masked
}
