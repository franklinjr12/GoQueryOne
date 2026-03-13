//go:build windows

package odbc

import (
	"fmt"
	"sort"
	"strings"

	"golang.org/x/sys/windows/registry"
)

func ListDSNs() ([]DSNEntry, error) {
	type dsnRoot struct {
		root  registry.Key
		path  string
		scope string
		arch  string
	}
	roots := []dsnRoot{
		{root: registry.CURRENT_USER, path: `Software\ODBC\ODBC.INI\ODBC Data Sources`, scope: "user", arch: "x64"},
		{root: registry.LOCAL_MACHINE, path: `Software\ODBC\ODBC.INI\ODBC Data Sources`, scope: "system", arch: "x64"},
		{root: registry.LOCAL_MACHINE, path: `Software\WOW6432Node\ODBC\ODBC.INI\ODBC Data Sources`, scope: "system", arch: "x86"},
	}

	result := make([]DSNEntry, 0)
	for _, item := range roots {
		key, err := registry.OpenKey(item.root, item.path, registry.READ)
		if err != nil {
			continue
		}
		names, readErr := key.ReadValueNames(-1)
		if readErr != nil {
			_ = key.Close()
			continue
		}
		for _, name := range names {
			driver, _, getErr := key.GetStringValue(name)
			if getErr != nil {
				driver = ""
			}
			result = append(result, DSNEntry{
				Name:         name,
				Driver:       driver,
				Scope:        item.scope,
				Architecture: item.arch,
			})
		}
		_ = key.Close()
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("no DSNs found")
	}
	sort.Slice(result, func(i, j int) bool {
		left := strings.ToLower(result[i].Name + result[i].Scope + result[i].Architecture)
		right := strings.ToLower(result[j].Name + result[j].Scope + result[j].Architecture)
		return left < right
	})
	return result, nil
}

func ListDrivers() ([]DriverEntry, error) {
	type driverRoot struct {
		root registry.Key
		path string
		arch string
	}
	roots := []driverRoot{
		{root: registry.LOCAL_MACHINE, path: `Software\ODBC\ODBCINST.INI\ODBC Drivers`, arch: "x64"},
		{root: registry.LOCAL_MACHINE, path: `Software\WOW6432Node\ODBC\ODBCINST.INI\ODBC Drivers`, arch: "x86"},
	}
	result := make([]DriverEntry, 0)
	for _, item := range roots {
		key, err := registry.OpenKey(item.root, item.path, registry.READ)
		if err != nil {
			continue
		}
		driverNames, readErr := key.ReadValueNames(-1)
		if readErr != nil {
			_ = key.Close()
			continue
		}
		for _, name := range driverNames {
			detailPath := strings.TrimSuffix(item.path, `\ODBC Drivers`) + `\` + name
			detailKey, detailErr := registry.OpenKey(item.root, detailPath, registry.READ)
			attrs := map[string]string{}
			if detailErr == nil {
				attrNames, _ := detailKey.ReadValueNames(-1)
				for _, attrName := range attrNames {
					val, _, getErr := detailKey.GetStringValue(attrName)
					if getErr == nil {
						attrs[attrName] = val
					}
				}
				_ = detailKey.Close()
			}
			result = append(result, DriverEntry{
				Name:         name,
				Architecture: item.arch,
				Attributes:   attrs,
			})
		}
		_ = key.Close()
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("no ODBC drivers found")
	}
	sort.Slice(result, func(i, j int) bool {
		left := strings.ToLower(result[i].Name + result[i].Architecture)
		right := strings.ToLower(result[j].Name + result[j].Architecture)
		return left < right
	})
	return result, nil
}
