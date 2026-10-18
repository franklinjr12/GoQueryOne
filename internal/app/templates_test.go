package app

import (
	"strings"
	"testing"

	"github.com/franklinjr12/GoQueryOne/internal/core"
)

func TestConnTemplates(t *testing.T) {
	names := ConnTemplateNames()
	if len(names) != len(ConnTemplates)+1 {
		t.Fatal(len(names))
	}
	for _, tpl := range ConnTemplates {
		if !strings.HasPrefix(tpl.Template, "Driver={") || !strings.HasSuffix(tpl.Template, ";") {
			t.Errorf("%s: malformed %q", tpl.Name, tpl.Template)
		}
		if len(core.UnfilledPlaceholders(tpl.Template)) == 0 {
			t.Errorf("%s: no placeholders", tpl.Name)
		}
		if strings.Contains(strings.ToLower(tpl.Template), "pwd=") {
			t.Errorf("%s: templates must not carry passwords", tpl.Name)
		}
	}
}
