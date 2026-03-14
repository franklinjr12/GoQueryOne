//go:build !windows

package ui

import (
	"errors"

	"github.com/franklinjr12/GoQueryOne/internal/config"
)

func Run(cfgPath string) error {
	return errors.New("GUI supported only on Windows")
}

func RunWithConfig(cfgPath string, cfg *config.Config) error {
	return errors.New("GUI supported only on Windows")
}
