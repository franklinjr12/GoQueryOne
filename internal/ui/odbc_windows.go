//go:build windows

package ui

import (
	"sort"
	"strings"

	"github.com/franklinjr12/GoQueryOne/internal/odbc"
	"github.com/franklinjr12/GoQueryOne/internal/odbcapi"
)

// newManager returns the ODBC manager with the native catalog enabled
// (SQLTables/SQLColumns/… through odbc32.dll).
func newManager() *odbc.Manager {
	m := odbc.NewManager()
	m.SetCatalogOpener(odbcapi.Open)
	return m
}

// dsnNames lists the DSNs usable by this exe's bitness: the driver manager's
// own list (SQLDataSources), falling back to the registry.
func dsnNames() []string {
	var names []string
	if dsns, err := odbcapi.DataSources(); err == nil && len(dsns) > 0 {
		for _, d := range dsns {
			names = append(names, d.Name)
		}
	} else if entries, regErr := odbc.ListDSNs(); regErr == nil {
		for _, e := range entries {
			names = append(names, e.Name)
		}
	}
	return uniqueSorted(names)
}

// driverNames lists installed ODBC drivers for this exe's bitness.
func driverNames() []string {
	var names []string
	if drivers, err := odbcapi.Drivers(); err == nil && len(drivers) > 0 {
		for _, d := range drivers {
			names = append(names, d.Name)
		}
	} else if entries, regErr := odbc.ListDrivers(); regErr == nil {
		for _, e := range entries {
			names = append(names, e.Name)
		}
	}
	return uniqueSorted(names)
}

// dsnDriver returns the driver registered for a DSN ("" if unknown).
func dsnDriver(dsn string) string {
	if dsns, err := odbcapi.DataSources(); err == nil {
		for _, d := range dsns {
			if strings.EqualFold(d.Name, dsn) {
				return d.Description
			}
		}
	}
	if entries, err := odbc.ListDSNs(); err == nil {
		for _, e := range entries {
			if strings.EqualFold(e.Name, dsn) {
				return e.Driver
			}
		}
	}
	return ""
}

func uniqueSorted(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, v := range in {
		k := strings.ToLower(v)
		if v == "" || seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i]) < strings.ToLower(out[j]) })
	return out
}
