package ui

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"
	"github.com/franklinjr12/GoQueryOne/internal/config"
	"github.com/franklinjr12/GoQueryOne/internal/odbc"
	"github.com/franklinjr12/GoQueryOne/internal/securestore"
)

const outputBufferLimit = 1 << 20

type sqlEditorTab struct {
	Item     *container.TabItem
	Editor   *widget.Entry
	Title    string
	FilePath string
	Dirty    bool
}

type applicationUI struct {
	app    fyne.App
	window fyne.Window

	configPath string
	cfg        *config.Config
	manager    *odbc.Manager
	secrets    *securestore.Store

	sessionSelect  *widget.Select
	statusLabel    *widget.Label
	editorTabs     *container.AppTabs
	editorTabIndex map[*container.TabItem]*sqlEditorTab

	resultsTable   *widget.Table
	resultsColumns []string
	resultsRows    [][]string
	selectedCell   widget.TableCellID
	loadMoreButton *widget.Button
	lastQuerySQL   string
	lastQueryArgs  []any
	lastMaxRows    int

	schemaSearchEntry *widget.Entry
	schemaList        *widget.List
	schemaDetails     *widget.Entry
	schemaTables      []odbc.SchemaTable
	selectedSchemaID  int

	messagesEntry *widget.Entry
	errorsEntry   *widget.Entry
	historyEntry  *widget.Entry

	outputEnabled bool

	sessionOptionMap map[string]string
	mu               sync.Mutex
}

func NewSimpleUI(a fyne.App) fyne.Window {
	cfgPath := config.ResolveConfigPath()
	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		cfg = config.DefaultConfig()
	}

	credPath := filepath.Join(filepath.Dir(cfgPath), "credentials.json")
	secrets, secretErr := securestore.New(credPath)
	if secretErr != nil {
		secrets = nil
	}

	ui := &applicationUI{
		app:              a,
		cfg:              cfg,
		configPath:       cfgPath,
		manager:          odbc.NewManager(),
		secrets:          secrets,
		editorTabIndex:   map[*container.TabItem]*sqlEditorTab{},
		resultsColumns:   []string{},
		resultsRows:      [][]string{},
		schemaTables:     []odbc.SchemaTable{},
		selectedSchemaID: -1,
		sessionOptionMap: map[string]string{},
		lastMaxRows:      cfg.Query.DefaultMaxRows,
	}

	ui.window = a.NewWindow("GoQueryOne")
	ui.window.Resize(fyne.NewSize(cfg.App.Window.Width, cfg.App.Window.Height))
	ui.build()
	ui.restoreProfiles()

	ui.window.SetCloseIntercept(func() {
		size := ui.window.Canvas().Size()
		ui.cfg.App.Window.Width = size.Width
		ui.cfg.App.Window.Height = size.Height
		_ = config.SaveConfig(ui.cfg, ui.configPath)
		ui.manager.DisconnectAll()
		ui.window.Close()
	})
	return ui.window
}

func (u *applicationUI) build() {
	u.statusLabel = widget.NewLabel("Disconnected")

	u.sessionSelect = widget.NewSelect([]string{}, func(selected string) {
		if sessionID, ok := u.sessionOptionMap[selected]; ok {
			_ = u.manager.SetActiveSession(sessionID)
			u.updateStatus()
		}
	})

	connectButton := widget.NewButton("Connect", func() {
		u.showConnectionWindow()
	})
	disconnectButton := widget.NewButton("Disconnect", func() {
		active, err := u.manager.ActiveSession()
		if err != nil {
			dialog.ShowInformation("Disconnect", "No active session.", u.window)
			return
		}
		if err := u.manager.Disconnect(active.ID); err != nil {
			dialog.ShowError(err, u.window)
			return
		}
		u.refreshSessionSelect()
		u.updateStatus()
	})
	odbcAdmin64 := widget.NewButton("ODBC Admin x64", func() {
		path, err := odbc.OpenODBCAdmin("x64")
		if err != nil {
			dialog.ShowError(fmt.Errorf("%w (path: %s)", err, path), u.window)
			return
		}
		dialog.ShowInformation("ODBC Admin", "Opened: "+path, u.window)
	})
	odbcAdmin32 := widget.NewButton("ODBC Admin x86", func() {
		path, err := odbc.OpenODBCAdmin("x86")
		if err != nil {
			dialog.ShowError(fmt.Errorf("%w (path: %s)", err, path), u.window)
			return
		}
		dialog.ShowInformation("ODBC Admin", "Opened: "+path, u.window)
	})
	aboutButton := widget.NewButton("About", func() {
		u.showAbout()
	})

	topBar := container.NewGridWithColumns(7,
		connectButton,
		disconnectButton,
		u.sessionSelect,
		odbcAdmin64,
		odbcAdmin32,
		aboutButton,
		widget.NewLabel(""),
	)

	editorPanel := u.buildEditorPanel()
	schemaPanel := u.buildSchemaPanel()
	resultsPanel := u.buildResultsPanel()

	rightSplit := container.NewVSplit(editorPanel, resultsPanel)
	rightSplit.SetOffset(0.45)

	mainSplit := container.NewHSplit(schemaPanel, rightSplit)
	mainSplit.SetOffset(0.28)

	root := container.NewBorder(topBar, u.statusLabel, nil, nil, mainSplit)
	u.window.SetContent(root)
}

func (u *applicationUI) buildEditorPanel() fyne.CanvasObject {
	newTabBtn := widget.NewButton("New Tab", func() {
		u.addEditorTab("SQL", "")
	})
	closeTabBtn := widget.NewButton("Close Tab", func() {
		u.closeCurrentTab()
	})
	renameTabBtn := widget.NewButton("Rename Tab", func() {
		u.renameCurrentTab()
	})
	openFileBtn := widget.NewButton("Open SQL", func() {
		u.openSQLFile()
	})
	saveFileBtn := widget.NewButton("Save SQL", func() {
		u.saveSQLFile()
	})
	runBtn := widget.NewButton("Run", func() {
		u.runStatement()
	})
	runScriptBtn := widget.NewButton("Run Script", func() {
		u.runScript()
	})
	cancelBtn := widget.NewButton("Cancel", func() {
		u.cancelExecution()
	})
	beginTxBtn := widget.NewButton("Begin Tx", func() {
		u.transactionAction("begin")
	})
	commitTxBtn := widget.NewButton("Commit", func() {
		u.transactionAction("commit")
	})
	rollbackTxBtn := widget.NewButton("Rollback", func() {
		u.transactionAction("rollback")
	})

	toolbar := container.NewGridWithColumns(11,
		newTabBtn, closeTabBtn, renameTabBtn, openFileBtn, saveFileBtn,
		runBtn, runScriptBtn, cancelBtn,
		beginTxBtn, commitTxBtn, rollbackTxBtn,
	)

	u.editorTabs = container.NewAppTabs()
	u.editorTabs.SetTabLocation(container.TabLocationTop)
	u.addEditorTab("SQL 1", "")

	return container.NewBorder(toolbar, nil, nil, nil, u.editorTabs)
}

func (u *applicationUI) addEditorTab(baseTitle, content string) {
	title := strings.TrimSpace(baseTitle)
	if title == "" {
		title = fmt.Sprintf("SQL %d", len(u.editorTabs.Items)+1)
	}
	entry := widget.NewMultiLineEntry()
	entry.SetText(content)
	entry.Wrapping = fyne.TextWrapOff

	tab := &sqlEditorTab{
		Editor: entry,
		Title:  title,
	}
	item := container.NewTabItem(title, container.NewScroll(entry))
	tab.Item = item
	u.editorTabIndex[item] = tab
	entry.OnChanged = func(_ string) {
		if tab.Dirty {
			return
		}
		tab.Dirty = true
		item.Text = title + " *"
		u.editorTabs.Refresh()
	}
	u.editorTabs.Append(item)
	u.editorTabs.Select(item)
}

func (u *applicationUI) currentEditorTab() *sqlEditorTab {
	selected := u.editorTabs.Selected()
	if selected == nil {
		return nil
	}
	return u.editorTabIndex[selected]
}

func (u *applicationUI) closeCurrentTab() {
	selected := u.editorTabs.Selected()
	if selected == nil {
		return
	}
	if len(u.editorTabs.Items) == 1 {
		tab := u.editorTabIndex[selected]
		tab.Editor.SetText("")
		tab.Dirty = false
		selected.Text = tab.Title
		u.editorTabs.Refresh()
		return
	}
	delete(u.editorTabIndex, selected)
	u.editorTabs.Remove(selected)
}

func (u *applicationUI) renameCurrentTab() {
	tab := u.currentEditorTab()
	if tab == nil {
		return
	}
	nameEntry := widget.NewEntry()
	nameEntry.SetText(tab.Title)
	dialog.ShowForm("Rename Tab", "Save", "Cancel", []*widget.FormItem{
		{Text: "Name", Widget: nameEntry},
	}, func(ok bool) {
		if !ok {
			return
		}
		newTitle := strings.TrimSpace(nameEntry.Text)
		if newTitle == "" {
			return
		}
		tab.Title = newTitle
		if tab.Dirty {
			tab.Item.Text = newTitle + " *"
		} else {
			tab.Item.Text = newTitle
		}
		u.editorTabs.Refresh()
	}, u.window)
}

func (u *applicationUI) openSQLFile() {
	d := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil {
			dialog.ShowError(err, u.window)
			return
		}
		if reader == nil {
			return
		}
		defer reader.Close()
		raw, readErr := io.ReadAll(reader)
		if readErr != nil {
			dialog.ShowError(readErr, u.window)
			return
		}
		name := filepath.Base(reader.URI().Path())
		u.addEditorTab(name, string(raw))
		tab := u.currentEditorTab()
		if tab != nil {
			tab.FilePath = reader.URI().Path()
			tab.Dirty = false
			tab.Item.Text = tab.Title
			u.editorTabs.Refresh()
		}
	}, u.window)
	d.SetFilter(storage.NewExtensionFileFilter([]string{".sql", ".txt"}))
	d.Show()
}

func (u *applicationUI) saveSQLFile() {
	tab := u.currentEditorTab()
	if tab == nil {
		return
	}
	save := func(writer fyne.URIWriteCloser, err error) {
		if err != nil {
			dialog.ShowError(err, u.window)
			return
		}
		if writer == nil {
			return
		}
		defer writer.Close()
		if _, writeErr := writer.Write([]byte(tab.Editor.Text)); writeErr != nil {
			dialog.ShowError(writeErr, u.window)
			return
		}
		tab.FilePath = writer.URI().Path()
		tab.Dirty = false
		tab.Title = filepath.Base(tab.FilePath)
		tab.Item.Text = tab.Title
		u.editorTabs.Refresh()
	}
	d := dialog.NewFileSave(save, u.window)
	d.SetFileName("query.sql")
	d.SetFilter(storage.NewExtensionFileFilter([]string{".sql", ".txt"}))
	d.Show()
}

func (u *applicationUI) buildSchemaPanel() fyne.CanvasObject {
	u.schemaSearchEntry = widget.NewEntry()
	u.schemaSearchEntry.SetPlaceHolder("Search table or column")
	u.schemaSearchEntry.OnChanged = func(_ string) {
		u.refreshSchemaList()
	}

	refreshBtn := widget.NewButton("Refresh", func() {
		u.loadSchema(true)
	})
	loadBtn := widget.NewButton("Load", func() {
		u.loadSchema(false)
	})

	u.schemaList = widget.NewList(
		func() int { return len(u.schemaTables) },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(id widget.ListItemID, item fyne.CanvasObject) {
			table := u.schemaTables[id]
			label := item.(*widget.Label)
			if table.Schema != "" {
				label.SetText(table.Schema + "." + table.Name + " [" + table.Type + "]")
			} else {
				label.SetText(table.Name + " [" + table.Type + "]")
			}
		},
	)
	u.schemaList.OnSelected = func(id widget.ListItemID) {
		u.selectedSchemaID = id
		if id >= 0 && id < len(u.schemaTables) {
			u.showTableDetails(u.schemaTables[id])
		}
	}

	selectTopBtn := widget.NewButton("SELECT TOP", func() {
		u.generateQuickQuery("top")
	})
	countBtn := widget.NewButton("COUNT(*)", func() {
		u.generateQuickQuery("count")
	})
	copyNameBtn := widget.NewButton("Copy Name", func() {
		selected := u.selectedSchemaTable()
		if selected == nil {
			return
		}
		u.window.Clipboard().SetContent(qualifiedName(*selected))
	})
	selectAllBtn := widget.NewButton("SELECT *", func() {
		u.generateQuickQuery("all")
	})

	u.schemaDetails = widget.NewMultiLineEntry()
	u.schemaDetails.Wrapping = fyne.TextWrapWord
	u.schemaDetails.Disable()

	top := container.NewBorder(nil, nil, nil, container.NewHBox(loadBtn, refreshBtn), u.schemaSearchEntry)
	actions := container.NewGridWithColumns(2, selectTopBtn, selectAllBtn, countBtn, copyNameBtn)
	split := container.NewVSplit(container.NewBorder(top, actions, nil, nil, u.schemaList), container.NewScroll(u.schemaDetails))
	split.SetOffset(0.63)
	return split
}

func (u *applicationUI) selectedSchemaTable() *odbc.SchemaTable {
	id := u.selectedSchemaID
	if id < 0 || id >= len(u.schemaTables) {
		return nil
	}
	table := u.schemaTables[id]
	return &table
}

func (u *applicationUI) buildResultsPanel() fyne.CanvasObject {
	u.messagesEntry = widget.NewMultiLineEntry()
	u.messagesEntry.Disable()
	u.errorsEntry = widget.NewMultiLineEntry()
	u.errorsEntry.Disable()
	u.historyEntry = widget.NewMultiLineEntry()
	u.historyEntry.Disable()

	u.resultsTable = widget.NewTable(
		func() (int, int) {
			return len(u.resultsRows) + 1, len(u.resultsColumns)
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("")
		},
		func(id widget.TableCellID, obj fyne.CanvasObject) {
			label := obj.(*widget.Label)
			if id.Row == 0 {
				if id.Col < len(u.resultsColumns) {
					label.SetText(u.resultsColumns[id.Col])
				} else {
					label.SetText("")
				}
				label.TextStyle = fyne.TextStyle{Bold: true}
				label.Refresh()
				return
			}
			row := id.Row - 1
			if row < len(u.resultsRows) && id.Col < len(u.resultsRows[row]) {
				label.SetText(u.resultsRows[row][id.Col])
			} else {
				label.SetText("")
			}
			label.TextStyle = fyne.TextStyle{}
			label.Refresh()
		},
	)
	u.resultsTable.OnSelected = func(id widget.TableCellID) {
		u.selectedCell = id
	}

	copySelectedBtn := widget.NewButton("Copy Selection", func() {
		u.copySelectedCell()
	})
	copyAllBtn := widget.NewButton("Copy All TSV", func() {
		u.copyAllRows("\t")
	})
	copyCSVBtn := widget.NewButton("Copy All CSV", func() {
		u.copyAllRows(",")
	})
	exportBtn := widget.NewButton("Export", func() {
		u.exportResults()
	})
	toggleOutputBtn := widget.NewButton("Toggle Text", func() {
		u.outputEnabled = !u.outputEnabled
		if u.outputEnabled {
			u.appendMessage("Text output enabled.")
		} else {
			u.appendMessage("Text output disabled.")
		}
	})
	copyErrorBtn := widget.NewButton("Copy Error", func() {
		u.window.Clipboard().SetContent(u.errorsEntry.Text)
	})
	u.loadMoreButton = widget.NewButton("Load More", func() {
		u.loadMoreRows()
	})
	u.loadMoreButton.Hide()

	toolbar := container.NewGridWithColumns(7,
		copySelectedBtn, copyAllBtn, copyCSVBtn, exportBtn, toggleOutputBtn, copyErrorBtn, u.loadMoreButton,
	)
	resultsTab := container.NewBorder(toolbar, nil, nil, nil, container.NewScroll(u.resultsTable))
	tabs := container.NewAppTabs(
		container.NewTabItem("Results", resultsTab),
		container.NewTabItem("Messages", container.NewScroll(u.messagesEntry)),
		container.NewTabItem("Errors", container.NewScroll(u.errorsEntry)),
		container.NewTabItem("History", container.NewScroll(u.historyEntry)),
	)
	tabs.SetTabLocation(container.TabLocationTop)
	return tabs
}

func (u *applicationUI) transactionAction(action string) {
	session, err := u.manager.ActiveSession()
	if err != nil {
		dialog.ShowInformation("Transaction", "No active connection.", u.window)
		return
	}
	var txErr error
	switch action {
	case "begin":
		txErr = u.manager.BeginTx(session.ID)
	case "commit":
		txErr = u.manager.CommitTx(session.ID)
	case "rollback":
		txErr = u.manager.RollbackTx(session.ID)
	}
	if txErr != nil {
		dialog.ShowError(txErr, u.window)
		return
	}
	u.appendMessage("Transaction " + action + " successful.")
}

func (u *applicationUI) runStatement() {
	session, err := u.manager.ActiveSession()
	if err != nil {
		dialog.ShowInformation("Run", "No active connection.", u.window)
		return
	}
	tab := u.currentEditorTab()
	if tab == nil {
		return
	}
	sqlText := strings.TrimSpace(tab.Editor.SelectedText())
	if sqlText == "" {
		sqlText = strings.TrimSpace(tab.Editor.Text)
	}
	if sqlText == "" {
		dialog.ShowInformation("Run", "No SQL to execute.", u.window)
		return
	}
	paramCount := odbc.CountPositionalParams(sqlText)
	run := func(params []any) {
		u.statusLabel.SetText("Executing...")
		u.lastQuerySQL = sqlText
		u.lastQueryArgs = params
		u.lastMaxRows = u.cfg.Query.DefaultMaxRows
		go func() {
			res, runErr := u.manager.ExecuteStatement(session.ID, sqlText, params, odbc.QueryOptions{
				Timeout:  time.Duration(u.cfg.Query.DefaultTimeoutMs) * time.Millisecond,
				MaxRows:  u.lastMaxRows,
				PageSize: u.cfg.Query.FetchPageSize,
			})
			u.runOnMain(func() {
				if runErr != nil {
					u.showDiagnostic(runErr, res)
				}
				u.applyStatementResult(res)
				u.updateHistory()
				u.updateStatus()
			})
		}()
	}
	if paramCount > 0 {
		u.promptParameters(paramCount, run)
		return
	}
	run(nil)
}

func (u *applicationUI) runScript() {
	session, err := u.manager.ActiveSession()
	if err != nil {
		dialog.ShowInformation("Run Script", "No active connection.", u.window)
		return
	}
	tab := u.currentEditorTab()
	if tab == nil {
		return
	}
	sqlText := strings.TrimSpace(tab.Editor.Text)
	if sqlText == "" {
		dialog.ShowInformation("Run Script", "No SQL script to execute.", u.window)
		return
	}
	u.statusLabel.SetText("Executing script...")
	go func() {
		res, runErr := u.manager.ExecuteScript(session.ID, sqlText, odbc.ScriptOptions{
			Timeout:   time.Duration(u.cfg.Query.DefaultTimeoutMs) * time.Millisecond,
			MaxRows:   u.cfg.Query.DefaultMaxRows,
			PageSize:  u.cfg.Query.FetchPageSize,
			StopOnErr: u.cfg.Query.StopOnErrorInScript,
		})
		u.runOnMain(func() {
			var b strings.Builder
			if runErr != nil {
				b.WriteString("Script error: " + runErr.Error() + "\n")
			}
			for i, item := range res.Results {
				status := "ok"
				if item.ErrorMessage != "" {
					status = "error"
				} else if item.Canceled {
					status = "canceled"
				} else if item.TimedOut {
					status = "timeout"
				}
				b.WriteString(fmt.Sprintf("%d. %s (%v)\n", i+1, status, item.ExecutionTime))
				if item.HasRows {
					b.WriteString(fmt.Sprintf("   rows: %d\n", item.ResultSet.RowCount))
				} else {
					b.WriteString(fmt.Sprintf("   rows affected: %d\n", item.RowsAffected))
				}
			}
			u.appendMessage(strings.TrimSpace(b.String()))
			for _, item := range res.Results {
				if item.HasRows {
					u.applyStatementResult(item)
					break
				}
			}
			if runErr != nil && len(res.Results) > 0 {
				last := res.Results[len(res.Results)-1]
				u.showDiagnostic(runErr, last)
			}
			u.updateHistory()
			u.updateStatus()
		})
	}()
}

func (u *applicationUI) cancelExecution() {
	session, err := u.manager.ActiveSession()
	if err != nil {
		return
	}
	if cancelErr := u.manager.CancelExecution(session.ID); cancelErr != nil {
		dialog.ShowInformation("Cancel", cancelErr.Error(), u.window)
		return
	}
	u.appendMessage("Cancel requested.")
}

func (u *applicationUI) applyStatementResult(res odbc.StatementResult) {
	if res.HasRows {
		u.resultsColumns = make([]string, len(res.ResultSet.Columns))
		for i, c := range res.ResultSet.Columns {
			u.resultsColumns[i] = c.Name
		}
		u.resultsRows = res.ResultSet.Rows
		u.resultsTable.Refresh()
		if res.ResultSet.Truncated {
			u.loadMoreButton.Show()
			u.appendMessage(fmt.Sprintf("Result truncated at %d rows.", res.ResultSet.TruncatedAt))
		} else {
			u.loadMoreButton.Hide()
		}
		u.appendMessage(fmt.Sprintf("Query completed in %v. Rows: %d, Cols: %d", res.ExecutionTime, res.ResultSet.RowCount, len(res.ResultSet.Columns)))
		if u.outputEnabled {
			u.appendMessage(FormatResultAsCSVLike(&res))
		}
		return
	}
	u.resultsColumns = []string{}
	u.resultsRows = [][]string{}
	u.resultsTable.Refresh()
	u.loadMoreButton.Hide()
	u.appendMessage(fmt.Sprintf("Command completed in %v. Rows affected: %d", res.ExecutionTime, res.RowsAffected))
}

func (u *applicationUI) showDiagnostic(err error, res odbc.StatementResult) {
	var b strings.Builder
	b.WriteString("Error: ")
	b.WriteString(err.Error())
	b.WriteString("\n")
	for _, rec := range res.Diagnostics {
		if rec.State != "" {
			b.WriteString("SQLSTATE=" + rec.State + " ")
		}
		if rec.NativeError != 0 {
			b.WriteString(fmt.Sprintf("Native=%d ", rec.NativeError))
		}
		b.WriteString(rec.Message)
		b.WriteString("\n")
	}
	u.errorsEntry.SetText(strings.TrimSpace(odbc.MaskSecrets(b.String())))
}

func (u *applicationUI) updateHistory() {
	session, err := u.manager.ActiveSession()
	if err != nil {
		u.historyEntry.SetText("")
		return
	}
	var b strings.Builder
	for _, item := range session.History {
		b.WriteString(fmt.Sprintf("%s [%s] %v\n%s\n\n", item.When.Format(time.RFC3339), item.Status, item.Duration, item.SQL))
	}
	u.historyEntry.SetText(strings.TrimSpace(b.String()))
}

func (u *applicationUI) appendMessage(msg string) {
	if strings.TrimSpace(msg) == "" {
		return
	}
	newText := strings.TrimSpace(u.messagesEntry.Text + "\n" + msg)
	if len(newText) > outputBufferLimit {
		newText = newText[len(newText)-outputBufferLimit:]
	}
	u.messagesEntry.SetText(newText)
}

func (u *applicationUI) loadSchema(forceRefresh bool) {
	session, err := u.manager.ActiveSession()
	if err != nil {
		dialog.ShowInformation("Schema", "No active connection.", u.window)
		return
	}
	u.statusLabel.SetText("Loading schema...")
	search := strings.TrimSpace(u.schemaSearchEntry.Text)
	go func() {
		var snapshot odbc.SchemaSnapshot
		var loadErr error
		if forceRefresh {
			snapshot, loadErr = u.manager.RefreshSchema(session.ID, search)
		} else {
			snapshot, loadErr = u.manager.LoadSchema(session.ID, search)
		}
		u.runOnMain(func() {
			if loadErr != nil {
				dialog.ShowError(loadErr, u.window)
				u.updateStatus()
				return
			}
			u.schemaTables = snapshot.Tables
			sort.Slice(u.schemaTables, func(i, j int) bool {
				return strings.ToLower(qualifiedName(u.schemaTables[i])) < strings.ToLower(qualifiedName(u.schemaTables[j]))
			})
			u.schemaList.Refresh()
			u.appendMessage(fmt.Sprintf("Schema loaded. %d tables/views.", len(u.schemaTables)))
			u.updateStatus()
		})
	}()
}

func (u *applicationUI) refreshSchemaList() {
	session, err := u.manager.ActiveSession()
	if err != nil {
		return
	}
	search := strings.TrimSpace(u.schemaSearchEntry.Text)
	snapshot, loadErr := u.manager.LoadSchema(session.ID, search)
	if loadErr != nil {
		return
	}
	u.schemaTables = snapshot.Tables
	u.schemaList.Refresh()
}

func (u *applicationUI) showTableDetails(table odbc.SchemaTable) {
	session, err := u.manager.ActiveSession()
	if err != nil {
		return
	}
	go func() {
		details, detailsErr := u.manager.TableDetails(session.ID, table.Catalog, table.Schema, table.Name)
		u.runOnMain(func() {
			if detailsErr != nil {
				u.schemaDetails.SetText(detailsErr.Error())
				return
			}
			var b strings.Builder
			b.WriteString("Columns:\n")
			for _, c := range details.Columns {
				nullability := "NOT NULL"
				if c.Nullable {
					nullability = "NULL"
				}
				b.WriteString(fmt.Sprintf("- %s %s %s\n", c.Name, c.Type, nullability))
			}
			b.WriteString("\nPrimary Keys:\n")
			if len(details.PrimaryKeys) == 0 {
				b.WriteString("- none\n")
			}
			for _, pk := range details.PrimaryKeys {
				b.WriteString("- " + pk + "\n")
			}
			b.WriteString("\nForeign Keys:\n")
			if len(details.ForeignKeys) == 0 {
				b.WriteString("- none\n")
			}
			for _, fk := range details.ForeignKeys {
				b.WriteString(fmt.Sprintf("- %s: %s -> %s.%s\n", fk.Name, fk.Column, fk.RefTable, fk.RefColumn))
			}
			b.WriteString("\nIndexes:\n")
			if len(details.Indexes) == 0 {
				b.WriteString("- none\n")
			}
			for _, idx := range details.Indexes {
				unique := "NONUNIQUE"
				if idx.Unique {
					unique = "UNIQUE"
				}
				b.WriteString(fmt.Sprintf("- %s (%s) %s\n", idx.Name, idx.Column, unique))
			}
			if len(details.Unsupported) > 0 {
				b.WriteString("\nNotes:\n")
				for _, message := range details.Unsupported {
					b.WriteString("- " + message + "\n")
				}
			}
			u.schemaDetails.SetText(strings.TrimSpace(b.String()))
		})
	}()
}

func (u *applicationUI) generateQuickQuery(mode string) {
	selected := u.selectedSchemaTable()
	if selected == nil {
		return
	}
	tab := u.currentEditorTab()
	if tab == nil {
		return
	}
	name := qualifiedName(*selected)
	switch mode {
	case "count":
		tab.Editor.SetText("SELECT COUNT(*) AS row_count FROM " + name + ";")
	case "top":
		session, err := u.manager.ActiveSession()
		limitSyntax := "TOP"
		if err == nil && session.Profile.Options.LimitSyntax != "" {
			limitSyntax = strings.ToUpper(session.Profile.Options.LimitSyntax)
		}
		if limitSyntax == "LIMIT" {
			tab.Editor.SetText("SELECT * FROM " + name + " LIMIT 100;")
		} else {
			tab.Editor.SetText("SELECT TOP 100 * FROM " + name + ";")
		}
	default:
		tab.Editor.SetText("SELECT * FROM " + name + ";")
	}
	tab.Dirty = true
	tab.Item.Text = tab.Title + " *"
	u.editorTabs.Refresh()
}

func (u *applicationUI) loadMoreRows() {
	session, err := u.manager.ActiveSession()
	if err != nil {
		return
	}
	if strings.TrimSpace(u.lastQuerySQL) == "" {
		return
	}
	u.lastMaxRows += u.cfg.Query.FetchPageSize
	u.statusLabel.SetText("Loading more...")
	go func() {
		res, runErr := u.manager.ExecuteStatement(session.ID, u.lastQuerySQL, u.lastQueryArgs, odbc.QueryOptions{
			Timeout:  time.Duration(u.cfg.Query.DefaultTimeoutMs) * time.Millisecond,
			MaxRows:  u.lastMaxRows,
			PageSize: u.cfg.Query.FetchPageSize,
		})
		u.runOnMain(func() {
			if runErr != nil {
				u.showDiagnostic(runErr, res)
				u.updateStatus()
				return
			}
			u.applyStatementResult(res)
			u.updateStatus()
		})
	}()
}

func (u *applicationUI) copySelectedCell() {
	if u.selectedCell.Row <= 0 || u.selectedCell.Col < 0 {
		return
	}
	row := u.selectedCell.Row - 1
	if row >= len(u.resultsRows) || u.selectedCell.Col >= len(u.resultsRows[row]) {
		return
	}
	u.window.Clipboard().SetContent(u.resultsRows[row][u.selectedCell.Col])
}

func (u *applicationUI) copyAllRows(delimiter string) {
	if len(u.resultsColumns) == 0 {
		return
	}
	lines := make([]string, 0, len(u.resultsRows)+1)
	lines = append(lines, strings.Join(u.resultsColumns, delimiter))
	for _, row := range u.resultsRows {
		clean := make([]string, len(row))
		for i, col := range row {
			clean[i] = strings.ReplaceAll(strings.ReplaceAll(col, "\r", " "), "\n", " ")
		}
		lines = append(lines, strings.Join(clean, delimiter))
	}
	u.window.Clipboard().SetContent(strings.Join(lines, "\n"))
}

func (u *applicationUI) exportResults() {
	if len(u.resultsColumns) == 0 {
		dialog.ShowInformation("Export", "No result set loaded.", u.window)
		return
	}
	save := dialog.NewFileSave(func(writer fyne.URIWriteCloser, err error) {
		if err != nil {
			dialog.ShowError(err, u.window)
			return
		}
		if writer == nil {
			return
		}
		defer writer.Close()
		ext := strings.ToLower(filepath.Ext(writer.URI().Path()))
		switch ext {
		case ".json":
			rows := make([]map[string]string, 0, len(u.resultsRows))
			for _, row := range u.resultsRows {
				item := map[string]string{}
				for i := range u.resultsColumns {
					if i < len(row) {
						item[u.resultsColumns[i]] = row[i]
					} else {
						item[u.resultsColumns[i]] = ""
					}
				}
				rows = append(rows, item)
			}
			enc := json.NewEncoder(writer)
			enc.SetIndent("", "  ")
			if encodeErr := enc.Encode(rows); encodeErr != nil {
				dialog.ShowError(encodeErr, u.window)
				return
			}
		case ".tsv":
			cw := csv.NewWriter(writer)
			cw.Comma = '\t'
			_ = cw.Write(u.resultsColumns)
			for _, row := range u.resultsRows {
				_ = cw.Write(row)
			}
			cw.Flush()
			if flushErr := cw.Error(); flushErr != nil {
				dialog.ShowError(flushErr, u.window)
				return
			}
		default:
			cw := csv.NewWriter(writer)
			_ = cw.Write(u.resultsColumns)
			for _, row := range u.resultsRows {
				_ = cw.Write(row)
			}
			cw.Flush()
			if flushErr := cw.Error(); flushErr != nil {
				dialog.ShowError(flushErr, u.window)
				return
			}
		}
		u.appendMessage("Export completed: " + writer.URI().Path())
	}, u.window)
	save.SetFileName("result.csv")
	save.SetFilter(storage.NewExtensionFileFilter([]string{".csv", ".tsv", ".json"}))
	save.Show()
}

func (u *applicationUI) showConnectionWindow() {
	win := u.app.NewWindow("Connection")
	win.Resize(fyne.NewSize(900, 620))

	modeSelect := widget.NewSelect([]string{"DSN", "Driver", "File DSN", "Connection string"}, nil)
	modeSelect.SetSelected("DSN")
	profileNameEntry := widget.NewEntry()
	profileNameEntry.SetPlaceHolder("Profile name")
	userEntry := widget.NewEntry()
	passwordEntry := widget.NewPasswordEntry()
	savePasswordCheck := widget.NewCheck("Save password securely", nil)
	savePasswordCheck.SetChecked(false)

	dsnSearchEntry := widget.NewEntry()
	includeUserCheck := widget.NewCheck("User", nil)
	includeSystemCheck := widget.NewCheck("System", nil)
	includeUserCheck.SetChecked(true)
	includeSystemCheck.SetChecked(true)
	dsnSelect := widget.NewSelect([]string{}, nil)
	driverSearchEntry := widget.NewEntry()
	driverSelect := widget.NewSelect([]string{}, nil)
	fileDsnEntry := widget.NewEntry()
	connectionStringEntry := widget.NewMultiLineEntry()
	connectionStringEntry.Wrapping = fyne.TextWrapWord
	finalConnectionString := widget.NewMultiLineEntry()
	finalConnectionString.Wrapping = fyne.TextWrapWord

	dsns := []odbc.DSNEntry{}
	drivers := []odbc.DriverEntry{}
	driverOptions := []string{}
	dsnOptions := []string{}
	dsnLabelMap := map[string]odbc.DSNEntry{}
	driverLabelMap := map[string]odbc.DriverEntry{}
	selectedProfileID := ""

	refreshDSNs := func() {
		filtered := odbc.FilterDSNs(dsns, dsnSearchEntry.Text, includeUserCheck.Checked, includeSystemCheck.Checked)
		dsnOptions = make([]string, 0, len(filtered))
		dsnLabelMap = map[string]odbc.DSNEntry{}
		for _, item := range filtered {
			label := fmt.Sprintf("%s (%s, %s)", item.Name, item.Scope, item.Architecture)
			dsnOptions = append(dsnOptions, label)
			dsnLabelMap[label] = item
		}
		dsnSelect.Options = dsnOptions
		if len(dsnOptions) > 0 {
			dsnSelect.SetSelected(dsnOptions[0])
		}
		dsnSelect.Refresh()
	}
	refreshDrivers := func() {
		filtered := odbc.FilterDrivers(drivers, driverSearchEntry.Text)
		driverOptions = make([]string, 0, len(filtered))
		driverLabelMap = map[string]odbc.DriverEntry{}
		for _, item := range filtered {
			label := fmt.Sprintf("%s (%s)", item.Name, item.Architecture)
			driverOptions = append(driverOptions, label)
			driverLabelMap[label] = item
		}
		driverSelect.Options = driverOptions
		if len(driverOptions) > 0 {
			driverSelect.SetSelected(driverOptions[0])
		}
		driverSelect.Refresh()
	}

	buildProfile := func() (config.ConnectionProfile, string, error) {
		name := strings.TrimSpace(profileNameEntry.Text)
		if name == "" {
			name = "Connection " + time.Now().Format("20060102150405")
		}
		profileID := selectedProfileID
		if profileID == "" {
			profileID = generateProfileID(name)
		}
		profile := config.ConnectionProfile{
			ID:           profileID,
			Name:         name,
			Type:         "dsn",
			Username:     strings.TrimSpace(userEntry.Text),
			SavePassword: savePasswordCheck.Checked,
			Options: config.ConnectionOptions{
				LoginTimeoutMs: 30000,
				LimitSyntax:    "TOP",
			},
		}
		password := passwordEntry.Text

		switch modeSelect.Selected {
		case "Driver":
			item, ok := driverLabelMap[driverSelect.Selected]
			if !ok {
				return profile, password, errors.New("select a driver")
			}
			profile.Type = "driver"
			profile.Driver = item.Name
		case "File DSN":
			if strings.TrimSpace(fileDsnEntry.Text) == "" {
				return profile, password, errors.New("file DSN path is required")
			}
			profile.Type = "file_dsn"
			profile.FilePath = strings.TrimSpace(fileDsnEntry.Text)
		case "Connection string":
			if strings.TrimSpace(connectionStringEntry.Text) == "" {
				return profile, password, errors.New("connection string is required")
			}
			profile.Type = "connection_string"
			profile.ConnectionString = strings.TrimSpace(connectionStringEntry.Text)
		default:
			item, ok := dsnLabelMap[dsnSelect.Selected]
			if !ok {
				return profile, password, errors.New("select a DSN")
			}
			profile.Type = "dsn"
			profile.DSN = item.Name
			if profile.Driver == "" {
				profile.Driver = item.Driver
			}
		}

		if profile.SavePassword && strings.TrimSpace(password) == "" {
			if existing, ok := u.cfg.ConnectionByID(profile.ID); ok && existing.CredentialRef != "" && u.secrets != nil {
				loaded, loadErr := u.secrets.Load(existing.CredentialRef)
				if loadErr == nil {
					password = loaded
				}
				profile.CredentialRef = existing.CredentialRef
			}
		}

		final, err := odbc.BuildConnectionString(profile, password)
		if err != nil {
			return profile, password, err
		}
		finalConnectionString.SetText(odbc.MaskSecrets(final))
		return profile, password, nil
	}

	modeContainers := map[string]fyne.CanvasObject{
		"dsn": container.NewVBox(
			widget.NewLabel("DSN search"),
			dsnSearchEntry,
			container.NewGridWithColumns(2, includeUserCheck, includeSystemCheck),
			dsnSelect,
		),
		"driver": container.NewVBox(
			widget.NewLabel("Driver search"),
			driverSearchEntry,
			driverSelect,
		),
		"file": container.NewVBox(
			widget.NewLabel("File DSN path"),
			fileDsnEntry,
		),
		"connstr": container.NewVBox(
			widget.NewLabel("Connection string"),
			connectionStringEntry,
		),
	}
	updateMode := func() {
		modeContainers["dsn"].Hide()
		modeContainers["driver"].Hide()
		modeContainers["file"].Hide()
		modeContainers["connstr"].Hide()
		switch modeSelect.Selected {
		case "Driver":
			modeContainers["driver"].Show()
		case "File DSN":
			modeContainers["file"].Show()
		case "Connection string":
			modeContainers["connstr"].Show()
		default:
			modeContainers["dsn"].Show()
		}
	}
	modeSelect.OnChanged = func(_ string) { updateMode() }
	dsnSearchEntry.OnChanged = func(_ string) { refreshDSNs() }
	driverSearchEntry.OnChanged = func(_ string) { refreshDrivers() }
	includeUserCheck.OnChanged = func(bool) { refreshDSNs() }
	includeSystemCheck.OnChanged = func(bool) { refreshDSNs() }

	loadDSNBtn := widget.NewButton("Load DSNs", func() {
		items, err := odbc.ListDSNs()
		if err != nil {
			dialog.ShowError(err, win)
			return
		}
		dsns = items
		refreshDSNs()
	})
	loadDriversBtn := widget.NewButton("Load Drivers", func() {
		items, err := odbc.ListDrivers()
		if err != nil {
			dialog.ShowError(err, win)
			return
		}
		drivers = items
		refreshDrivers()
	})

	profileOptions := make([]string, 0, len(u.cfg.Connections))
	profileLabelMap := map[string]config.ConnectionProfile{}
	for _, p := range u.cfg.Connections {
		label := p.Name + " [" + p.Type + "]"
		profileOptions = append(profileOptions, label)
		profileLabelMap[label] = p
	}
	profileSelect := widget.NewSelect(profileOptions, nil)
	loadProfileBtn := widget.NewButton("Load Profile", func() {
		p, ok := profileLabelMap[profileSelect.Selected]
		if !ok {
			return
		}
		selectedProfileID = p.ID
		profileNameEntry.SetText(p.Name)
		userEntry.SetText(p.Username)
		savePasswordCheck.SetChecked(p.SavePassword)
		switch p.Type {
		case "driver":
			modeSelect.SetSelected("Driver")
			for label, driver := range driverLabelMap {
				if driver.Name == p.Driver {
					driverSelect.SetSelected(label)
					break
				}
			}
		case "file_dsn":
			modeSelect.SetSelected("File DSN")
			fileDsnEntry.SetText(p.FilePath)
		case "connection_string":
			modeSelect.SetSelected("Connection string")
			connectionStringEntry.SetText(p.ConnectionString)
		default:
			modeSelect.SetSelected("DSN")
			for label, entry := range dsnLabelMap {
				if entry.Name == p.DSN {
					dsnSelect.SetSelected(label)
					break
				}
			}
		}
	})
	deleteProfileBtn := widget.NewButton("Delete Profile", func() {
		p, ok := profileLabelMap[profileSelect.Selected]
		if !ok {
			return
		}
		u.cfg.RemoveConnection(p.ID)
		if p.CredentialRef != "" && u.secrets != nil {
			_ = u.secrets.Delete(p.CredentialRef)
		}
		if err := config.SaveConfig(u.cfg, u.configPath); err != nil {
			dialog.ShowError(err, win)
			return
		}
		dialog.ShowInformation("Profiles", "Profile deleted.", win)
		win.Close()
		u.showConnectionWindow()
	})

	saveProfile := func(profile config.ConnectionProfile, password string) error {
		if profile.SavePassword {
			if u.secrets == nil {
				return errors.New("secure store is unavailable; password was not saved")
			}
			ref := profile.CredentialRef
			if ref == "" {
				ref = "dpapi:GoQueryOne:" + profile.ID
			}
			if strings.TrimSpace(password) == "" {
				return errors.New("password is empty")
			}
			if err := u.secrets.Save(ref, password); err != nil {
				return err
			}
			profile.CredentialRef = ref
		} else if profile.CredentialRef != "" && u.secrets != nil {
			_ = u.secrets.Delete(profile.CredentialRef)
			profile.CredentialRef = ""
		}
		u.cfg.UpsertConnection(profile)
		return config.SaveConfig(u.cfg, u.configPath)
	}

	testBtn := widget.NewButton("Test", func() {
		profile, password, err := buildProfile()
		if err != nil {
			dialog.ShowError(err, win)
			return
		}
		go func() {
			elapsed, testErr := u.manager.TestConnection(profile, password, time.Duration(profile.Options.LoginTimeoutMs)*time.Millisecond)
			u.runOnMain(func() {
				if testErr != nil {
					dialog.ShowError(testErr, win)
					return
				}
				dialog.ShowInformation("Connection Test", "Success in "+elapsed.String(), win)
			})
		}()
	})

	saveBtn := widget.NewButton("Save Profile", func() {
		profile, password, err := buildProfile()
		if err != nil {
			dialog.ShowError(err, win)
			return
		}
		if saveErr := saveProfile(profile, password); saveErr != nil {
			dialog.ShowError(saveErr, win)
			return
		}
		dialog.ShowInformation("Profiles", "Profile saved.", win)
	})

	connectBtn := widget.NewButton("Connect", func() {
		profile, password, err := buildProfile()
		if err != nil {
			dialog.ShowError(err, win)
			return
		}
		go func() {
			_, connectErr := u.manager.Connect(profile, password, time.Duration(profile.Options.LoginTimeoutMs)*time.Millisecond)
			u.runOnMain(func() {
				if connectErr != nil {
					dialog.ShowError(connectErr, win)
					return
				}
				if saveErr := saveProfile(profile, password); saveErr != nil {
					dialog.ShowError(saveErr, win)
					return
				}
				u.refreshSessionSelect()
				u.updateStatus()
				win.Close()
			})
		}()
	})

	cancelBtn := widget.NewButton("Cancel", func() { win.Close() })

	leftPanel := container.NewVBox(
		widget.NewLabel("Saved profiles"),
		profileSelect,
		container.NewGridWithColumns(2, loadProfileBtn, deleteProfileBtn),
		widget.NewSeparator(),
		widget.NewLabel("Mode"),
		modeSelect,
		modeContainers["dsn"],
		modeContainers["driver"],
		modeContainers["file"],
		modeContainers["connstr"],
		container.NewGridWithColumns(2, loadDSNBtn, loadDriversBtn),
	)

	rightPanel := container.NewVBox(
		widget.NewLabel("Profile name"),
		profileNameEntry,
		widget.NewLabel("Username"),
		userEntry,
		widget.NewLabel("Password"),
		passwordEntry,
		savePasswordCheck,
		widget.NewLabel("Final connection string"),
		finalConnectionString,
		container.NewGridWithColumns(4, testBtn, saveBtn, connectBtn, cancelBtn),
	)

	updateMode()
	refreshDSNs()
	refreshDrivers()
	win.SetContent(container.NewHSplit(container.NewScroll(leftPanel), container.NewScroll(rightPanel)))
	win.Show()
}

func (u *applicationUI) showAbout() {
	var report strings.Builder
	report.WriteString("GoQueryOne\n")
	report.WriteString("Architecture: " + runtime.GOARCH + "\n")
	report.WriteString("Config path: " + u.configPath + "\n")
	report.WriteString("Portable mode: " + strconv.FormatBool(strings.Contains(strings.ToLower(u.configPath), strings.ToLower(filepath.Dir(executablePath())))) + "\n")
	if session, err := u.manager.ActiveSession(); err == nil {
		report.WriteString("Active connection: " + session.Name + "\n")
		report.WriteString("State: " + string(session.State) + "\n")
		if len(session.ConnectedInfo) > 0 {
			keys := make([]string, 0, len(session.ConnectedInfo))
			for k := range session.ConnectedInfo {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				report.WriteString(fmt.Sprintf("%s: %s\n", k, session.ConnectedInfo[k]))
			}
		}
	}
	dialog.ShowInformation("About", report.String(), u.window)
}

func (u *applicationUI) refreshSessionSelect() {
	sessions := u.manager.SessionList()
	sort.Slice(sessions, func(i, j int) bool {
		return strings.ToLower(sessions[i].Name) < strings.ToLower(sessions[j].Name)
	})
	options := make([]string, 0, len(sessions))
	u.sessionOptionMap = map[string]string{}
	selected := ""
	active, _ := u.manager.ActiveSession()
	for _, s := range sessions {
		label := fmt.Sprintf("%s (%s)", s.Name, s.State)
		options = append(options, label)
		u.sessionOptionMap[label] = s.ID
		if active != nil && active.ID == s.ID {
			selected = label
		}
	}
	u.sessionSelect.Options = options
	u.sessionSelect.Refresh()
	if selected != "" {
		u.sessionSelect.SetSelected(selected)
	} else if len(options) > 0 {
		u.sessionSelect.SetSelected(options[0])
	}
}

func (u *applicationUI) updateStatus() {
	session, err := u.manager.ActiveSession()
	if err != nil {
		u.statusLabel.SetText("Disconnected")
		return
	}
	msg := fmt.Sprintf("Active: %s | State: %s", session.Name, session.State)
	if session.Tx != nil {
		msg += " | Transaction: ON"
	}
	if !session.LastQueryAt.IsZero() {
		msg += " | Last Query: " + session.LastQueryAt.Format(time.RFC3339)
	}
	u.statusLabel.SetText(msg)
}

func (u *applicationUI) restoreProfiles() {
	u.refreshSessionSelect()
	u.updateStatus()
}

func (u *applicationUI) promptParameters(count int, callback func([]any)) {
	win := u.app.NewWindow("Parameters")
	win.Resize(fyne.NewSize(550, 420))
	type paramInput struct {
		Type  *widget.Select
		Value *widget.Entry
	}
	inputs := make([]paramInput, 0, count)
	rows := make([]fyne.CanvasObject, 0, count)
	for i := 0; i < count; i++ {
		typeSelect := widget.NewSelect([]string{"Auto", "String", "Int", "Float", "Bool", "Null"}, nil)
		typeSelect.SetSelected("Auto")
		valueEntry := widget.NewEntry()
		row := container.NewGridWithColumns(3,
			widget.NewLabel(fmt.Sprintf("Param %d", i+1)),
			typeSelect,
			valueEntry,
		)
		rows = append(rows, row)
		inputs = append(inputs, paramInput{Type: typeSelect, Value: valueEntry})
	}
	runBtn := widget.NewButton("Run", func() {
		values := make([]any, 0, count)
		for _, input := range inputs {
			raw := input.Value.Text
			switch input.Type.Selected {
			case "Null":
				values = append(values, nil)
			case "String":
				values = append(values, raw)
			case "Int":
				iv, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
				if err != nil {
					dialog.ShowError(err, win)
					return
				}
				values = append(values, iv)
			case "Float":
				fv, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
				if err != nil {
					dialog.ShowError(err, win)
					return
				}
				values = append(values, fv)
			case "Bool":
				bv, err := strconv.ParseBool(strings.TrimSpace(raw))
				if err != nil {
					dialog.ShowError(err, win)
					return
				}
				values = append(values, bv)
			default:
				values = append(values, raw)
			}
		}
		win.Close()
		callback(values)
	})
	cancelBtn := widget.NewButton("Cancel", func() { win.Close() })
	content := container.NewBorder(nil, container.NewGridWithColumns(2, runBtn, cancelBtn), nil, nil, container.NewVScroll(container.NewVBox(rows...)))
	win.SetContent(content)
	win.Show()
}

func (u *applicationUI) runOnMain(fn func()) {
	if fn == nil {
		return
	}
	fyne.Do(fn)
}

func generateProfileID(name string) string {
	clean := strings.ToLower(strings.TrimSpace(name))
	clean = strings.ReplaceAll(clean, " ", "-")
	if clean == "" {
		clean = "connection"
	}
	return fmt.Sprintf("%s-%d", clean, time.Now().UnixNano())
}

func qualifiedName(t odbc.SchemaTable) string {
	if strings.TrimSpace(t.Schema) == "" {
		return t.Name
	}
	return t.Schema + "." + t.Name
}

func executablePath() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return exe
}
