package main

import (
	"io"
	"log"
	"os"

	"fyne.io/fyne/v2/app"
	"github.com/franklinjr12/GoQueryOne/internal/ui"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	// set the file as logs.txt
	logFile, err := os.Create("logs.txt")
	if err != nil {
		log.Fatalf("Error creating logs.txt: %v", err)
	}
	defer logFile.Close()
	log.SetOutput(io.MultiWriter(os.Stdout, logFile))

	a := app.New()
	w := ui.NewSimpleUI(a)
	w.ShowAndRun()
}
