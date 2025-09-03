package ui

import (
	"context"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/franklinjr12/GoQueryOne/internal/odbc"
)

// NewSimpleUI constructs the minimal UI and wires up events. It returns a ready window.
func NewSimpleUI(a fyne.App) fyne.Window {
	w := a.NewWindow("GoQueryOne")
	w.Resize(fyne.NewSize(900, 600))

	// Widgets
	dsnEntry := widget.NewEntry()
	dsnEntry.SetPlaceHolder("ODBC DSN")

	connectButton := widget.NewButton("Connect", nil)

	queryEntry := widget.NewMultiLineEntry()
	queryEntry.SetPlaceHolder("SELECT 1")
	queryEntry.Wrapping = fyne.TextWrapWord

	runButton := widget.NewButton("Run Query", nil)
	runButton.Disable()

	resultsLabel := widget.NewLabel("")
	resultsLabel.Wrapping = fyne.TextWrapWord
	resultsScroller := container.NewVScroll(resultsLabel)

	// State
	var conn *odbc.Connection
	var connected bool

	// Connect/Disconnect handler
	connectButton.OnTapped = func() {
		if !connected {
			dsn := dsnEntry.Text
			if dsn == "" {
				dialog.ShowInformation("Info", "Please enter a DSN.", w)
				return
			}
			connectButton.Disable()
			go func() {
				c := odbc.NewConnection(dsn)
				err := c.Connect()
				if err != nil {
					connectButton.Enable()
					dialog.ShowError(err, w)
					return
				}
				conn = c
				connected = true
				connectButton.SetText("Disconnect")
				connectButton.Enable()
				runButton.Enable()
			}()
		} else {
			connectButton.Disable()
			go func() {
				var err error
				if conn != nil {
					err = conn.Disconnect()
				}
				if err != nil {
					dialog.ShowError(err, w)
				}
				conn = nil
				connected = false
				connectButton.SetText("Connect")
				connectButton.Enable()
				runButton.Disable()
			}()
		}
	}

	// Run query handler
	runButton.OnTapped = func() {
		if !connected || conn == nil {
			dialog.ShowInformation("Info", "Not connected.", w)
			return
		}
		query := queryEntry.Text
		if query == "" {
			dialog.ShowInformation("Info", "Please enter a query.", w)
			return
		}
		runButton.Disable()
		resultsLabel.SetText("Running...")
		go func() {
			// Optional: context timeout for safety in future iterations
			_ = context.Background()
			start := time.Now()
			res, err := conn.ExecuteQuery(query)
			elapsed := time.Since(start)
			_ = elapsed
			runButton.Enable()
			if err != nil {
				dialog.ShowError(err, w)
				return
			}
			// Use formatter
			// No truncation for now (maxRows=0)
			text := FormatResultAsCSVLike(res, 0)
			resultsLabel.SetText(text)
		}()
	}

	// Ensure disconnect on close
	w.SetCloseIntercept(func() {
		if connected && conn != nil {
			go func() {
				_ = conn.Disconnect()
				w.Close()
			}()
			return
		}
		w.Close()
	})

	// Layout
	topRow := container.NewBorder(nil, nil, nil, connectButton, dsnEntry)
	actionRow := container.NewBorder(nil, nil, nil, runButton)
	content := container.NewBorder(
		container.NewVBox(topRow, queryEntry, actionRow),
		nil,
		nil,
		nil,
		resultsScroller,
	)

	w.SetContent(content)
	return w
}
