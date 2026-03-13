package main

import (
	"io"
	"log"
	"os"
	"path/filepath"

	"fyne.io/fyne/v2/app"
	"github.com/franklinjr12/GoQueryOne/internal/config"
	"github.com/franklinjr12/GoQueryOne/internal/ui"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	cfgPath := config.ResolveConfigPath()
	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		cfg = config.DefaultConfig()
	}
	logPath := cfg.App.Logging.File
	if filepath.Dir(logPath) == "." {
		logPath = filepath.Join(filepath.Dir(cfgPath), logPath)
	}
	logFile, err := openLogFile(logPath, cfg.App.Logging.MaxMiB)
	if err != nil {
		log.Fatalf("Error opening log file %s: %v", logPath, err)
	}
	defer logFile.Close()
	log.SetOutput(io.MultiWriter(os.Stdout, logFile))

	a := app.New()
	w := ui.NewSimpleUI(a)
	w.ShowAndRun()
}

func openLogFile(path string, maxMiB int) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	if maxMiB <= 0 {
		maxMiB = 20
	}
	if stat, err := os.Stat(path); err == nil {
		if stat.Size() > int64(maxMiB)*1024*1024 {
			backup := path + ".1"
			_ = os.Remove(backup)
			_ = os.Rename(path, backup)
		}
	}
	return os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
}
