//go:build windows

package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"

	"github.com/franklinjr12/GoQueryOne/internal/buildinfo"
	"github.com/franklinjr12/GoQueryOne/internal/config"
	_ "github.com/franklinjr12/GoQueryOne/internal/odbcdriver"
	"github.com/franklinjr12/GoQueryOne/internal/ui"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	cfgPath := config.ResolveConfigPath()
	cfg, notice := config.LoadOrRecover(cfgPath, time.Now())
	dir := filepath.Dir(cfgPath)

	logPath := cfg.App.Logging.File
	if logPath == "" {
		logPath = "logs.txt"
	}
	if filepath.Dir(logPath) == "." {
		logPath = filepath.Join(dir, logPath)
	}
	if logFile, err := openLogFile(logPath, cfg.App.Logging.MaxMiB); err != nil {
		// A read-only folder must not stop the app: log to stdout only.
		log.Printf("cannot open log file %s: %v", logPath, err)
	} else {
		defer logFile.Close()
		log.SetOutput(io.MultiWriter(os.Stdout, logFile))
	}

	// Fatal runtime errors (also panics in background goroutines) are
	// written to crash.log next to the config.
	crashPath := filepath.Join(dir, "crash.log")
	if f, err := os.OpenFile(crashPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644); err == nil {
		_ = debug.SetCrashOutput(f, debug.CrashOptions{})
		_ = f.Close()
	}
	defer func() {
		if r := recover(); r != nil {
			msg := fmt.Sprintf("%s\npanic: %v\n\n%s", time.Now().Format(time.RFC3339), r, debug.Stack())
			log.Print(msg)
			appendFile(crashPath, msg)
			ui.ShowMessage("GoQueryOne", fmt.Sprintf("GoQueryOne stopped because of an unexpected error:\n\n%v\n\nDetails were written to %s.", r, crashPath), true)
			os.Exit(2)
		}
	}()

	log.Printf("starting %s, config %s", buildinfo.String(), cfgPath)
	if notice != "" {
		log.Print(notice)
		ui.ShowMessage("GoQueryOne settings", notice, true)
	}

	if err := ui.RunWithConfig(cfgPath, cfg); err != nil {
		log.Printf("error starting UI: %v", err)
		ui.ShowMessage("GoQueryOne", "GoQueryOne could not start:\n\n"+err.Error(), true)
		os.Exit(1)
	}
	// Clean exit: drop the crash log if nothing was written to it.
	_ = debug.SetCrashOutput(nil, debug.CrashOptions{}) // release the handle first (Windows)
	if st, err := os.Stat(crashPath); err == nil && st.Size() == 0 {
		_ = os.Remove(crashPath)
	}
}

func appendFile(path, text string) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.WriteString(text + "\n")
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
