//go:build windows

package ui

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/franklinjr12/GoQueryOne/internal/config"
	"github.com/franklinjr12/GoQueryOne/internal/odbc"
	"github.com/franklinjr12/GoQueryOne/internal/securestore"
	"github.com/lxn/walk"
	"github.com/lxn/walk/declarative"
)

const outputBufferLimit = 1 << 20

type sqlEditorTab struct {
	Page          *walk.TabPage
	Editor        *walk.TextEdit
	Title         string
	FilePath      string
	Dirty         bool
	suppressDirty bool
}

type resultsTableModel struct {
	walk.TableModelBase
	columns []string
	rows    [][]string
}

func (m *resultsTableModel) RowCount() int {
	return len(m.rows)
}

func (m *resultsTableModel) Value(row, col int) interface{} {
	if row < 0 || row >= len(m.rows) {
		return ""
	}
	if col < 0 || col >= len(m.columns) {
		return ""
	}
	if col >= len(m.rows[row]) {
		return ""
	}
	return m.rows[row][col]
}

func (m *resultsTableModel) SetData(columns []string, rows [][]string) {
	m.columns = append([]string(nil), columns...)
	m.rows = make([][]string, len(rows))
	for i := range rows {
		m.rows[i] = append([]string(nil), rows[i]...)
	}
	m.PublishRowsReset()
}

type applicationUI struct {
	mainWindow *walk.MainWindow

	configPath string
	cfg        *config.Config
	manager    *odbc.Manager
	secrets    *securestore.Store

	sessionSelect    *walk.ComboBox
	sessionOptions   []string
	sessionOptionMap map[string]string
	statusLabel      *walk.Label

	editorTabs     *walk.TabWidget
	editorTabIndex map[*walk.TabPage]*sqlEditorTab

	resultsTable   *walk.TableView
	resultsModel   *resultsTableModel
	resultsColumns []string
	resultsRows    [][]string
	selectedRow    int
	selectedCol    int
	loadMoreButton *walk.PushButton
	lastQuerySQL   string
	lastQueryArgs  []any
	lastMaxRows    int

	schemaSearchEntry *walk.LineEdit
	schemaList        *walk.ListBox
	schemaDetails     *walk.TextEdit
	schemaTables      []odbc.SchemaTable
	selectedSchemaID  int

	messagesEntry *walk.TextEdit
	errorsEntry   *walk.TextEdit
	historyEntry  *walk.TextEdit

	outputEnabled bool
}

func Run(cfgPath string) error {
	if strings.TrimSpace(cfgPath) == "" {
		cfgPath = config.ResolveConfigPath()
	}
	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		cfg = config.DefaultConfig()
	}
	return RunWithConfig(cfgPath, cfg)
}

func RunWithConfig(cfgPath string, cfg *config.Config) error {
	if strings.TrimSpace(cfgPath) == "" {
		cfgPath = config.ResolveConfigPath()
	}
	if cfg == nil {
		loaded, err := config.LoadConfig(cfgPath)
		if err != nil {
			cfg = config.DefaultConfig()
		} else {
			cfg = loaded
		}
	}

	credPath := filepath.Join(filepath.Dir(cfgPath), "credentials.json")
	secrets, secretErr := securestore.New(credPath)
	if secretErr != nil {
		secrets = nil
	}

	ui := &applicationUI{
		configPath:       cfgPath,
		cfg:              cfg,
		manager:          odbc.NewManager(),
		secrets:          secrets,
		sessionOptionMap: map[string]string{},
		editorTabIndex:   map[*walk.TabPage]*sqlEditorTab{},
		resultsModel:     &resultsTableModel{},
		resultsColumns:   []string{},
		resultsRows:      [][]string{},
		schemaTables:     []odbc.SchemaTable{},
		selectedSchemaID: -1,
		selectedRow:      -1,
		selectedCol:      0,
		lastMaxRows:      cfg.Query.DefaultMaxRows,
	}

	return ui.run()
}

func (u *applicationUI) run() error {
	width := int(u.cfg.App.Window.Width)
	height := int(u.cfg.App.Window.Height)
	if width <= 0 {
		width = 1260
	}
	if height <= 0 {
		height = 760
	}

	mainWindow := declarative.MainWindow{
		AssignTo: &u.mainWindow,
		Title:    "GoQueryOne",
		Bounds: declarative.Rectangle{
			X:      int(u.cfg.App.Window.X),
			Y:      int(u.cfg.App.Window.Y),
			Width:  width,
			Height: height,
		},
		MinSize: declarative.Size{Width: 1024, Height: 640},
		Layout:  declarative.VBox{MarginsZero: true, Spacing: 6},
		Visible: false,
	}
	if err := mainWindow.Create(); err != nil {
		return err
	}

	if err := u.build(); err != nil {
		return err
	}
	u.restoreProfiles()
	u.mainWindow.Show()
	u.mainWindow.Run()
	return nil
}

func (u *applicationUI) build() error {
	layout, ok := u.mainWindow.Layout().(*walk.BoxLayout)
	if !ok {
		return errors.New("main window layout initialization failed")
	}
	_ = layout.SetMargins(walk.Margins{6, 6, 6, 6})
	_ = layout.SetSpacing(6)

	topBar, err := u.buildTopBar()
	if err != nil {
		return err
	}
	_ = topBar.SetMinMaxSize(walk.Size{Height: 38}, walk.Size{Height: 38})

	mainSplit, err := walk.NewHSplitter(u.mainWindow)
	if err != nil {
		return err
	}

	schemaPanel, err := u.buildSchemaPanel(mainSplit)
	if err != nil {
		return err
	}
	_ = schemaPanel.SetMinMaxSize(walk.Size{Width: 280}, walk.Size{})

	rightSplit, err := walk.NewVSplitter(mainSplit)
	if err != nil {
		return err
	}

	editorPanel, err := u.buildEditorPanel(rightSplit)
	if err != nil {
		return err
	}
	_ = editorPanel.SetMinMaxSize(walk.Size{Height: 260}, walk.Size{})

	if _, err := u.buildResultsPanel(rightSplit); err != nil {
		return err
	}

	u.statusLabel, err = walk.NewLabel(u.mainWindow)
	if err != nil {
		return err
	}
	_ = u.statusLabel.SetText("Disconnected")
	_ = u.statusLabel.SetMinMaxSize(walk.Size{Height: 22}, walk.Size{Height: 22})

	_ = layout.SetStretchFactor(mainSplit, 1)

	u.mainWindow.Closing().Attach(func(canceled *bool, reason walk.CloseReason) {
		b := u.mainWindow.BoundsPixels()
		u.cfg.App.Window.Width = float32(b.Width)
		u.cfg.App.Window.Height = float32(b.Height)
		u.cfg.App.Window.X = float32(b.X)
		u.cfg.App.Window.Y = float32(b.Y)
		_ = config.SaveConfig(u.cfg, u.configPath)
		u.manager.DisconnectAll()
	})

	return nil
}

func (u *applicationUI) buildTopBar() (*walk.Composite, error) {
	topBar, err := walk.NewComposite(u.mainWindow)
	if err != nil {
		return nil, err
	}
	layout := walk.NewHBoxLayout()
	_ = layout.SetMargins(walk.Margins{})
	_ = layout.SetSpacing(6)
	if err := topBar.SetLayout(layout); err != nil {
		return nil, err
	}

	connectButton, err := walk.NewPushButton(topBar)
	if err != nil {
		return nil, err
	}
	_ = connectButton.SetText("Connect")
	connectButton.Clicked().Attach(func() {
		u.showConnectionWindow()
	})

	disconnectButton, err := walk.NewPushButton(topBar)
	if err != nil {
		return nil, err
	}
	_ = disconnectButton.SetText("Disconnect")
	disconnectButton.Clicked().Attach(func() {
		active, activeErr := u.manager.ActiveSession()
		if activeErr != nil {
			showInfoBox(u.mainWindow, "Disconnect", "No active session.")
			return
		}
		if err := u.manager.Disconnect(active.ID); err != nil {
			showErrorBox(u.mainWindow, "Disconnect", err)
			return
		}
		u.refreshSessionSelect()
		u.updateStatus()
	})

	u.sessionSelect, err = walk.NewDropDownBox(topBar)
	if err != nil {
		return nil, err
	}
	_ = u.sessionSelect.SetModel([]string{})
	_ = layout.SetStretchFactor(u.sessionSelect, 2)

	u.sessionSelect.CurrentIndexChanged().Attach(func() {
		idx := u.sessionSelect.CurrentIndex()
		if idx < 0 || idx >= len(u.sessionOptions) {
			return
		}
		label := u.sessionOptions[idx]
		sessionID, ok := u.sessionOptionMap[label]
		if !ok {
			return
		}
		if err := u.manager.SetActiveSession(sessionID); err != nil {
			u.refreshSessionSelect()
			showErrorBox(u.mainWindow, "Session", err)
			return
		}
		u.updateStatus()
	})

	odbcAdmin64, err := walk.NewPushButton(topBar)
	if err != nil {
		return nil, err
	}
	_ = odbcAdmin64.SetText("ODBC Admin x64")
	odbcAdmin64.Clicked().Attach(func() {
		path, openErr := odbc.OpenODBCAdmin("x64")
		if openErr != nil {
			showErrorBox(u.mainWindow, "ODBC Admin", fmt.Errorf("%w (path: %s)", openErr, path))
			return
		}
		showInfoBox(u.mainWindow, "ODBC Admin", "Opened: "+path)
	})

	odbcAdmin32, err := walk.NewPushButton(topBar)
	if err != nil {
		return nil, err
	}
	_ = odbcAdmin32.SetText("ODBC Admin x86")
	odbcAdmin32.Clicked().Attach(func() {
		path, openErr := odbc.OpenODBCAdmin("x86")
		if openErr != nil {
			showErrorBox(u.mainWindow, "ODBC Admin", fmt.Errorf("%w (path: %s)", openErr, path))
			return
		}
		showInfoBox(u.mainWindow, "ODBC Admin", "Opened: "+path)
	})

	aboutButton, err := walk.NewPushButton(topBar)
	if err != nil {
		return nil, err
	}
	_ = aboutButton.SetText("About")
	aboutButton.Clicked().Attach(func() {
		u.showAbout()
	})

	spacer, err := walk.NewComposite(topBar)
	if err != nil {
		return nil, err
	}
	_ = spacer.SetMinMaxSize(walk.Size{Width: 8}, walk.Size{})
	_ = layout.SetStretchFactor(spacer, 1)

	return topBar, nil
}

func (u *applicationUI) buildEditorPanel(parent walk.Container) (*walk.Composite, error) {
	panel, err := walk.NewComposite(parent)
	if err != nil {
		return nil, err
	}
	panelLayout := walk.NewVBoxLayout()
	_ = panelLayout.SetMargins(walk.Margins{})
	_ = panelLayout.SetSpacing(6)
	if err := panel.SetLayout(panelLayout); err != nil {
		return nil, err
	}

	toolbar, err := walk.NewComposite(panel)
	if err != nil {
		return nil, err
	}
	toolbarLayout := walk.NewHBoxLayout()
	_ = toolbarLayout.SetMargins(walk.Margins{})
	_ = toolbarLayout.SetSpacing(6)
	if err := toolbar.SetLayout(toolbarLayout); err != nil {
		return nil, err
	}

	buttons := []struct {
		label   string
		handler func()
	}{
		{"New Tab", func() { u.addEditorTab("SQL", "") }},
		{"Close Tab", u.closeCurrentTab},
		{"Rename Tab", u.renameCurrentTab},
		{"Open SQL", u.openSQLFile},
		{"Save SQL", u.saveSQLFile},
		{"Run", u.runStatement},
		{"Run Script", u.runScript},
		{"Cancel", u.cancelExecution},
		{"Begin Tx", func() { u.transactionAction("begin") }},
		{"Commit", func() { u.transactionAction("commit") }},
		{"Rollback", func() { u.transactionAction("rollback") }},
	}

	for _, item := range buttons {
		btn, newErr := walk.NewPushButton(toolbar)
		if newErr != nil {
			return nil, newErr
		}
		_ = btn.SetText(item.label)
		btn.Clicked().Attach(item.handler)
	}

	u.editorTabs, err = walk.NewTabWidget(panel)
	if err != nil {
		return nil, err
	}
	_ = panelLayout.SetStretchFactor(u.editorTabs, 1)

	u.addEditorTab("SQL 1", "")
	return panel, nil
}

func (u *applicationUI) addEditorTab(baseTitle, content string) {
	if u.editorTabs == nil {
		return
	}
	title := strings.TrimSpace(baseTitle)
	if title == "" {
		title = fmt.Sprintf("SQL %d", u.editorTabs.Pages().Len()+1)
	}

	page, err := walk.NewTabPage()
	if err != nil {
		showErrorBox(u.mainWindow, "Editor", err)
		return
	}
	_ = page.SetTitle(title)

	layout := walk.NewVBoxLayout()
	_ = layout.SetMargins(walk.Margins{})
	_ = layout.SetSpacing(0)
	if err := page.SetLayout(layout); err != nil {
		showErrorBox(u.mainWindow, "Editor", err)
		return
	}

	editor, err := walk.NewTextEdit(page)
	if err != nil {
		showErrorBox(u.mainWindow, "Editor", err)
		return
	}
	_ = editor.SetText(content)
	_ = layout.SetStretchFactor(editor, 1)

	tab := &sqlEditorTab{Page: page, Editor: editor, Title: title}
	u.editorTabIndex[page] = tab

	editor.TextChanged().Attach(func() {
		if tab.suppressDirty || tab.Dirty {
			return
		}
		tab.Dirty = true
		u.refreshTabTitle(tab)
	})

	if err := u.editorTabs.Pages().Add(page); err != nil {
		showErrorBox(u.mainWindow, "Editor", err)
		return
	}
	_ = u.editorTabs.SetCurrentIndex(u.editorTabs.Pages().Len() - 1)
}

func (u *applicationUI) refreshTabTitle(tab *sqlEditorTab) {
	if tab == nil || tab.Page == nil {
		return
	}
	title := tab.Title
	if tab.Dirty {
		title += " *"
	}
	_ = tab.Page.SetTitle(title)
}

func (u *applicationUI) currentEditorTab() *sqlEditorTab {
	if u.editorTabs == nil {
		return nil
	}
	idx := u.editorTabs.CurrentIndex()
	if idx < 0 || idx >= u.editorTabs.Pages().Len() {
		return nil
	}
	return u.editorTabIndex[u.editorTabs.Pages().At(idx)]
}

func (u *applicationUI) closeCurrentTab() {
	if u.editorTabs == nil {
		return
	}
	idx := u.editorTabs.CurrentIndex()
	if idx < 0 || idx >= u.editorTabs.Pages().Len() {
		return
	}
	page := u.editorTabs.Pages().At(idx)
	tab := u.editorTabIndex[page]
	if tab == nil {
		return
	}

	if u.editorTabs.Pages().Len() == 1 {
		tab.suppressDirty = true
		_ = tab.Editor.SetText("")
		tab.suppressDirty = false
		tab.Dirty = false
		u.refreshTabTitle(tab)
		return
	}

	delete(u.editorTabIndex, page)
	_ = u.editorTabs.Pages().Remove(page)
}

func (u *applicationUI) renameCurrentTab() {
	tab := u.currentEditorTab()
	if tab == nil {
		return
	}
	name, ok := u.promptLine("Rename Tab", "Name", tab.Title)
	if !ok {
		return
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}
	tab.Title = name
	u.refreshTabTitle(tab)
}

func (u *applicationUI) openSQLFile() {
	dlg := walk.FileDialog{
		Title:  "Open SQL",
		Filter: "SQL Files (*.sql;*.txt)|*.sql;*.txt|All Files (*.*)|*.*",
	}
	accepted, err := dlg.ShowOpen(u.mainWindow)
	if err != nil {
		showErrorBox(u.mainWindow, "Open SQL", err)
		return
	}
	if !accepted {
		return
	}

	raw, err := os.ReadFile(dlg.FilePath)
	if err != nil {
		showErrorBox(u.mainWindow, "Open SQL", err)
		return
	}

	name := filepath.Base(dlg.FilePath)
	u.addEditorTab(name, string(raw))
	tab := u.currentEditorTab()
	if tab != nil {
		tab.FilePath = dlg.FilePath
		tab.Title = name
		tab.Dirty = false
		u.refreshTabTitle(tab)
	}
}

func (u *applicationUI) saveSQLFile() {
	tab := u.currentEditorTab()
	if tab == nil {
		return
	}

	dlg := walk.FileDialog{
		Title:       "Save SQL",
		FilePath:    tab.FilePath,
		Filter:      "SQL Files (*.sql;*.txt)|*.sql;*.txt|All Files (*.*)|*.*",
		FilterIndex: 1,
	}
	if strings.TrimSpace(dlg.FilePath) == "" {
		dlg.FilePath = "query.sql"
	}

	accepted, err := dlg.ShowSave(u.mainWindow)
	if err != nil {
		showErrorBox(u.mainWindow, "Save SQL", err)
		return
	}
	if !accepted {
		return
	}

	if err := os.WriteFile(dlg.FilePath, []byte(tab.Editor.Text()), 0o644); err != nil {
		showErrorBox(u.mainWindow, "Save SQL", err)
		return
	}

	tab.FilePath = dlg.FilePath
	tab.Title = filepath.Base(dlg.FilePath)
	tab.Dirty = false
	u.refreshTabTitle(tab)
}
func (u *applicationUI) buildSchemaPanel(parent walk.Container) (*walk.Composite, error) {
	panel, err := walk.NewComposite(parent)
	if err != nil {
		return nil, err
	}
	layout := walk.NewVBoxLayout()
	_ = layout.SetMargins(walk.Margins{})
	_ = layout.SetSpacing(6)
	if err := panel.SetLayout(layout); err != nil {
		return nil, err
	}

	split, err := walk.NewVSplitter(panel)
	if err != nil {
		return nil, err
	}
	_ = layout.SetStretchFactor(split, 1)

	topSection, err := walk.NewComposite(split)
	if err != nil {
		return nil, err
	}
	topLayout := walk.NewVBoxLayout()
	_ = topLayout.SetMargins(walk.Margins{})
	_ = topLayout.SetSpacing(6)
	if err := topSection.SetLayout(topLayout); err != nil {
		return nil, err
	}

	u.schemaSearchEntry, err = walk.NewLineEdit(topSection)
	if err != nil {
		return nil, err
	}
	_ = u.schemaSearchEntry.SetCueBanner("Search table or column")
	u.schemaSearchEntry.TextChanged().Attach(func() {
		u.refreshSchemaList()
	})

	buttonRow, err := walk.NewComposite(topSection)
	if err != nil {
		return nil, err
	}
	buttonRowLayout := walk.NewHBoxLayout()
	_ = buttonRowLayout.SetMargins(walk.Margins{})
	_ = buttonRowLayout.SetSpacing(6)
	if err := buttonRow.SetLayout(buttonRowLayout); err != nil {
		return nil, err
	}

	loadBtn, err := walk.NewPushButton(buttonRow)
	if err != nil {
		return nil, err
	}
	_ = loadBtn.SetText("Load")
	loadBtn.Clicked().Attach(func() {
		u.loadSchema(false)
	})

	refreshBtn, err := walk.NewPushButton(buttonRow)
	if err != nil {
		return nil, err
	}
	_ = refreshBtn.SetText("Refresh")
	refreshBtn.Clicked().Attach(func() {
		u.loadSchema(true)
	})

	u.schemaList, err = walk.NewListBox(topSection)
	if err != nil {
		return nil, err
	}
	_ = topLayout.SetStretchFactor(u.schemaList, 1)
	_ = u.schemaList.SetModel([]string{})
	u.schemaList.CurrentIndexChanged().Attach(func() {
		idx := u.schemaList.CurrentIndex()
		u.selectedSchemaID = idx
		if idx >= 0 && idx < len(u.schemaTables) {
			u.showTableDetails(u.schemaTables[idx])
		}
	})

	actionsRow1, err := walk.NewComposite(topSection)
	if err != nil {
		return nil, err
	}
	row1Layout := walk.NewHBoxLayout()
	_ = row1Layout.SetMargins(walk.Margins{})
	_ = row1Layout.SetSpacing(6)
	if err := actionsRow1.SetLayout(row1Layout); err != nil {
		return nil, err
	}

	selectTopBtn, err := walk.NewPushButton(actionsRow1)
	if err != nil {
		return nil, err
	}
	_ = selectTopBtn.SetText("SELECT TOP")
	selectTopBtn.Clicked().Attach(func() {
		u.generateQuickQuery("top")
	})

	selectAllBtn, err := walk.NewPushButton(actionsRow1)
	if err != nil {
		return nil, err
	}
	_ = selectAllBtn.SetText("SELECT *")
	selectAllBtn.Clicked().Attach(func() {
		u.generateQuickQuery("all")
	})

	actionsRow2, err := walk.NewComposite(topSection)
	if err != nil {
		return nil, err
	}
	row2Layout := walk.NewHBoxLayout()
	_ = row2Layout.SetMargins(walk.Margins{})
	_ = row2Layout.SetSpacing(6)
	if err := actionsRow2.SetLayout(row2Layout); err != nil {
		return nil, err
	}

	countBtn, err := walk.NewPushButton(actionsRow2)
	if err != nil {
		return nil, err
	}
	_ = countBtn.SetText("COUNT(*)")
	countBtn.Clicked().Attach(func() {
		u.generateQuickQuery("count")
	})

	copyNameBtn, err := walk.NewPushButton(actionsRow2)
	if err != nil {
		return nil, err
	}
	_ = copyNameBtn.SetText("Copy Name")
	copyNameBtn.Clicked().Attach(func() {
		selected := u.selectedSchemaTable()
		if selected == nil {
			return
		}
		_ = walk.Clipboard().SetText(qualifiedName(*selected))
	})

	u.schemaDetails, err = walk.NewTextEdit(split)
	if err != nil {
		return nil, err
	}
	_ = u.schemaDetails.SetReadOnly(true)

	return panel, nil
}

func (u *applicationUI) selectedSchemaTable() *odbc.SchemaTable {
	id := u.selectedSchemaID
	if id < 0 || id >= len(u.schemaTables) {
		return nil
	}
	table := u.schemaTables[id]
	return &table
}

func (u *applicationUI) buildResultsPanel(parent walk.Container) (*walk.Composite, error) {
	panel, err := walk.NewComposite(parent)
	if err != nil {
		return nil, err
	}
	layout := walk.NewVBoxLayout()
	_ = layout.SetMargins(walk.Margins{})
	_ = layout.SetSpacing(0)
	if err := panel.SetLayout(layout); err != nil {
		return nil, err
	}

	tabs, err := walk.NewTabWidget(panel)
	if err != nil {
		return nil, err
	}
	_ = layout.SetStretchFactor(tabs, 1)

	resultsPage, err := walk.NewTabPage()
	if err != nil {
		return nil, err
	}
	_ = resultsPage.SetTitle("Results")
	resultsLayout := walk.NewVBoxLayout()
	_ = resultsLayout.SetMargins(walk.Margins{})
	_ = resultsLayout.SetSpacing(6)
	if err := resultsPage.SetLayout(resultsLayout); err != nil {
		return nil, err
	}

	toolbar, err := walk.NewComposite(resultsPage)
	if err != nil {
		return nil, err
	}
	toolbarLayout := walk.NewHBoxLayout()
	_ = toolbarLayout.SetMargins(walk.Margins{})
	_ = toolbarLayout.SetSpacing(6)
	if err := toolbar.SetLayout(toolbarLayout); err != nil {
		return nil, err
	}

	copySelectedBtn, err := walk.NewPushButton(toolbar)
	if err != nil {
		return nil, err
	}
	_ = copySelectedBtn.SetText("Copy Selection")
	copySelectedBtn.Clicked().Attach(func() {
		u.copySelectedCell()
	})

	copyAllBtn, err := walk.NewPushButton(toolbar)
	if err != nil {
		return nil, err
	}
	_ = copyAllBtn.SetText("Copy All TSV")
	copyAllBtn.Clicked().Attach(func() {
		u.copyAllRows("\t")
	})

	copyCSVBtn, err := walk.NewPushButton(toolbar)
	if err != nil {
		return nil, err
	}
	_ = copyCSVBtn.SetText("Copy All CSV")
	copyCSVBtn.Clicked().Attach(func() {
		u.copyAllRows(",")
	})

	exportBtn, err := walk.NewPushButton(toolbar)
	if err != nil {
		return nil, err
	}
	_ = exportBtn.SetText("Export")
	exportBtn.Clicked().Attach(func() {
		u.exportResults()
	})

	toggleOutputBtn, err := walk.NewPushButton(toolbar)
	if err != nil {
		return nil, err
	}
	_ = toggleOutputBtn.SetText("Toggle Text")
	toggleOutputBtn.Clicked().Attach(func() {
		u.outputEnabled = !u.outputEnabled
		if u.outputEnabled {
			u.appendMessage("Text output enabled.")
		} else {
			u.appendMessage("Text output disabled.")
		}
	})

	copyErrorBtn, err := walk.NewPushButton(toolbar)
	if err != nil {
		return nil, err
	}
	_ = copyErrorBtn.SetText("Copy Error")
	copyErrorBtn.Clicked().Attach(func() {
		_ = walk.Clipboard().SetText(u.errorsEntry.Text())
	})

	u.loadMoreButton, err = walk.NewPushButton(toolbar)
	if err != nil {
		return nil, err
	}
	_ = u.loadMoreButton.SetText("Load More")
	u.loadMoreButton.Clicked().Attach(func() {
		u.loadMoreRows()
	})
	u.loadMoreButton.SetVisible(false)

	u.resultsTable, err = walk.NewTableView(resultsPage)
	if err != nil {
		return nil, err
	}
	if err := u.resultsTable.SetModel(u.resultsModel); err != nil {
		return nil, err
	}
	_ = u.resultsTable.SetLastColumnStretched(true)
	_ = u.resultsTable.SetMultiSelection(false)
	u.resultsTable.CurrentIndexChanged().Attach(func() {
		u.selectedRow = u.resultsTable.CurrentIndex()
		u.selectedCol = 0
	})
	_ = resultsLayout.SetStretchFactor(u.resultsTable, 1)

	u.rebuildResultsColumns()

	messagesPage, err := walk.NewTabPage()
	if err != nil {
		return nil, err
	}
	_ = messagesPage.SetTitle("Messages")
	if u.messagesEntry, err = u.newReadOnlyTextPage(messagesPage); err != nil {
		return nil, err
	}

	errorsPage, err := walk.NewTabPage()
	if err != nil {
		return nil, err
	}
	_ = errorsPage.SetTitle("Errors")
	if u.errorsEntry, err = u.newReadOnlyTextPage(errorsPage); err != nil {
		return nil, err
	}

	historyPage, err := walk.NewTabPage()
	if err != nil {
		return nil, err
	}
	_ = historyPage.SetTitle("History")
	if u.historyEntry, err = u.newReadOnlyTextPage(historyPage); err != nil {
		return nil, err
	}

	if err := tabs.Pages().Add(resultsPage); err != nil {
		return nil, err
	}
	if err := tabs.Pages().Add(messagesPage); err != nil {
		return nil, err
	}
	if err := tabs.Pages().Add(errorsPage); err != nil {
		return nil, err
	}
	if err := tabs.Pages().Add(historyPage); err != nil {
		return nil, err
	}

	return panel, nil
}

func (u *applicationUI) newReadOnlyTextPage(page *walk.TabPage) (*walk.TextEdit, error) {
	layout := walk.NewVBoxLayout()
	_ = layout.SetMargins(walk.Margins{})
	_ = layout.SetSpacing(0)
	if err := page.SetLayout(layout); err != nil {
		return nil, err
	}
	entry, err := walk.NewTextEdit(page)
	if err != nil {
		return nil, err
	}
	_ = entry.SetReadOnly(true)
	_ = layout.SetStretchFactor(entry, 1)
	return entry, nil
}

func (u *applicationUI) rebuildResultsColumns() {
	if u.resultsTable == nil {
		return
	}
	cols := u.resultsTable.Columns()
	_ = cols.Clear()
	if len(u.resultsColumns) == 0 {
		col := walk.NewTableViewColumn()
		_ = col.SetTitle("Result")
		col.SetWidth(180)
		_ = cols.Add(col)
		return
	}
	for _, name := range u.resultsColumns {
		col := walk.NewTableViewColumn()
		_ = col.SetTitle(name)
		col.SetWidth(140)
		_ = cols.Add(col)
	}
}

func (u *applicationUI) transactionAction(action string) {
	session, err := u.manager.ActiveSession()
	if err != nil {
		showInfoBox(u.mainWindow, "Transaction", "No active connection.")
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
		showErrorBox(u.mainWindow, "Transaction", txErr)
		return
	}
	u.appendMessage("Transaction " + action + " successful.")
}

func (u *applicationUI) runStatement() {
	session, err := u.manager.ActiveSession()
	if err != nil {
		showInfoBox(u.mainWindow, "Run", "No active connection.")
		return
	}
	tab := u.currentEditorTab()
	if tab == nil {
		return
	}

	sqlText := strings.TrimSpace(selectedText(tab.Editor))
	if sqlText == "" {
		sqlText = strings.TrimSpace(tab.Editor.Text())
	}
	if sqlText == "" {
		showInfoBox(u.mainWindow, "Run", "No SQL to execute.")
		return
	}

	paramCount := odbc.CountPositionalParams(sqlText)
	run := func(params []any) {
		_ = u.statusLabel.SetText("Executing...")
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
		showInfoBox(u.mainWindow, "Run Script", "No active connection.")
		return
	}
	tab := u.currentEditorTab()
	if tab == nil {
		return
	}
	sqlText := strings.TrimSpace(tab.Editor.Text())
	if sqlText == "" {
		showInfoBox(u.mainWindow, "Run Script", "No SQL script to execute.")
		return
	}

	_ = u.statusLabel.SetText("Executing script...")
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
				u.showDiagnostic(runErr, res.Results[len(res.Results)-1])
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
		showInfoBox(u.mainWindow, "Cancel", cancelErr.Error())
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
		u.rebuildResultsColumns()
		u.resultsModel.SetData(u.resultsColumns, u.resultsRows)
		if res.ResultSet.Truncated {
			u.loadMoreButton.SetVisible(true)
			u.appendMessage(fmt.Sprintf("Result truncated at %d rows.", res.ResultSet.TruncatedAt))
		} else {
			u.loadMoreButton.SetVisible(false)
		}
		u.appendMessage(fmt.Sprintf("Query completed in %v. Rows: %d, Cols: %d", res.ExecutionTime, res.ResultSet.RowCount, len(res.ResultSet.Columns)))
		if u.outputEnabled {
			u.appendMessage(FormatResultAsCSVLike(&res))
		}
		return
	}

	u.resultsColumns = []string{}
	u.resultsRows = [][]string{}
	u.rebuildResultsColumns()
	u.resultsModel.SetData(u.resultsColumns, u.resultsRows)
	u.loadMoreButton.SetVisible(false)
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
	_ = u.errorsEntry.SetText(strings.TrimSpace(odbc.MaskSecrets(b.String())))
}

func (u *applicationUI) updateHistory() {
	session, err := u.manager.ActiveSession()
	if err != nil {
		_ = u.historyEntry.SetText("")
		return
	}
	var b strings.Builder
	for _, item := range session.History {
		b.WriteString(fmt.Sprintf("%s [%s] %v\n%s\n\n", item.When.Format(time.RFC3339), item.Status, item.Duration, item.SQL))
	}
	_ = u.historyEntry.SetText(strings.TrimSpace(b.String()))
}

func (u *applicationUI) appendMessage(msg string) {
	if strings.TrimSpace(msg) == "" {
		return
	}
	newText := strings.TrimSpace(u.messagesEntry.Text() + "\n" + msg)
	if len(newText) > outputBufferLimit {
		newText = newText[len(newText)-outputBufferLimit:]
	}
	_ = u.messagesEntry.SetText(newText)
}
func (u *applicationUI) loadSchema(forceRefresh bool) {
	session, err := u.manager.ActiveSession()
	if err != nil {
		showInfoBox(u.mainWindow, "Schema", "No active connection.")
		return
	}

	_ = u.statusLabel.SetText("Loading schema...")
	search := strings.TrimSpace(u.schemaSearchEntry.Text())

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
				showErrorBox(u.mainWindow, "Schema", loadErr)
				u.updateStatus()
				return
			}
			u.schemaTables = snapshot.Tables
			sort.Slice(u.schemaTables, func(i, j int) bool {
				return strings.ToLower(qualifiedName(u.schemaTables[i])) < strings.ToLower(qualifiedName(u.schemaTables[j]))
			})
			u.refreshSchemaWidget()
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
	snapshot, loadErr := u.manager.LoadSchema(session.ID, strings.TrimSpace(u.schemaSearchEntry.Text()))
	if loadErr != nil {
		return
	}
	u.schemaTables = snapshot.Tables
	sort.Slice(u.schemaTables, func(i, j int) bool {
		return strings.ToLower(qualifiedName(u.schemaTables[i])) < strings.ToLower(qualifiedName(u.schemaTables[j]))
	})
	u.refreshSchemaWidget()
}

func (u *applicationUI) refreshSchemaWidget() {
	options := make([]string, 0, len(u.schemaTables))
	for _, table := range u.schemaTables {
		if table.Schema != "" {
			options = append(options, table.Schema+"."+table.Name+" ["+table.Type+"]")
		} else {
			options = append(options, table.Name+" ["+table.Type+"]")
		}
	}
	_ = u.schemaList.SetModel(options)
	u.selectedSchemaID = -1
	_ = u.schemaDetails.SetText("")
	if len(options) > 0 {
		_ = u.schemaList.SetCurrentIndex(0)
	}
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
				_ = u.schemaDetails.SetText(detailsErr.Error())
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
			_ = u.schemaDetails.SetText(strings.TrimSpace(b.String()))
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
	query := "SELECT * FROM " + name + ";"
	switch mode {
	case "count":
		query = "SELECT COUNT(*) AS row_count FROM " + name + ";"
	case "top":
		limitSyntax := "TOP"
		if session, err := u.manager.ActiveSession(); err == nil && session.Profile.Options.LimitSyntax != "" {
			limitSyntax = strings.ToUpper(session.Profile.Options.LimitSyntax)
		}
		if limitSyntax == "LIMIT" {
			query = "SELECT * FROM " + name + " LIMIT 100;"
		} else {
			query = "SELECT TOP 100 * FROM " + name + ";"
		}
	}

	tab.suppressDirty = true
	_ = tab.Editor.SetText(query)
	tab.suppressDirty = false
	tab.Dirty = true
	u.refreshTabTitle(tab)
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
	_ = u.statusLabel.SetText("Loading more...")

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
	if len(u.resultsRows) == 0 {
		return
	}
	row := u.resultsTable.CurrentIndex()
	if row < 0 || row >= len(u.resultsRows) {
		return
	}
	col := u.selectedCol
	if col < 0 || col >= len(u.resultsRows[row]) {
		col = 0
	}
	if len(u.resultsRows[row]) == 0 {
		return
	}
	_ = walk.Clipboard().SetText(u.resultsRows[row][col])
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
	_ = walk.Clipboard().SetText(strings.Join(lines, "\n"))
}

func (u *applicationUI) exportResults() {
	if len(u.resultsColumns) == 0 {
		showInfoBox(u.mainWindow, "Export", "No result set loaded.")
		return
	}

	dlg := walk.FileDialog{
		Title:       "Export Results",
		FilePath:    "result.csv",
		Filter:      "CSV (*.csv)|*.csv|TSV (*.tsv)|*.tsv|JSON (*.json)|*.json",
		FilterIndex: 1,
	}
	accepted, err := dlg.ShowSave(u.mainWindow)
	if err != nil {
		showErrorBox(u.mainWindow, "Export", err)
		return
	}
	if !accepted {
		return
	}

	file, err := os.Create(dlg.FilePath)
	if err != nil {
		showErrorBox(u.mainWindow, "Export", err)
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(dlg.FilePath))
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
		enc := json.NewEncoder(file)
		enc.SetIndent("", "  ")
		if err := enc.Encode(rows); err != nil {
			showErrorBox(u.mainWindow, "Export", err)
			return
		}
	case ".tsv":
		writer := csv.NewWriter(file)
		writer.Comma = '\t'
		_ = writer.Write(u.resultsColumns)
		for _, row := range u.resultsRows {
			_ = writer.Write(row)
		}
		writer.Flush()
		if err := writer.Error(); err != nil {
			showErrorBox(u.mainWindow, "Export", err)
			return
		}
	default:
		writer := csv.NewWriter(file)
		_ = writer.Write(u.resultsColumns)
		for _, row := range u.resultsRows {
			_ = writer.Write(row)
		}
		writer.Flush()
		if err := writer.Error(); err != nil {
			showErrorBox(u.mainWindow, "Export", err)
			return
		}
	}

	u.appendMessage("Export completed: " + dlg.FilePath)
}

func (u *applicationUI) showConnectionWindow() {
	dlg, err := walk.NewDialog(u.mainWindow)
	if err != nil {
		showErrorBox(u.mainWindow, "Connection", err)
		return
	}
	_ = dlg.SetTitle("Connection")
	_ = dlg.SetBounds(walk.Rectangle{Width: 760, Height: 460})

	layout := walk.NewGridLayout()
	_ = layout.SetMargins(walk.Margins{10, 10, 10, 10})
	_ = layout.SetSpacing(8)
	if err := dlg.SetLayout(layout); err != nil {
		showErrorBox(u.mainWindow, "Connection", err)
		return
	}

	profileLabel, _ := walk.NewLabel(dlg)
	_ = profileLabel.SetText("Profile")
	_ = layout.SetRange(profileLabel, walk.Rectangle{X: 0, Y: 0, Width: 1, Height: 1})

	profileSelect, _ := walk.NewDropDownBox(dlg)
	_ = layout.SetRange(profileSelect, walk.Rectangle{X: 1, Y: 0, Width: 3, Height: 1})

	modeLabel, _ := walk.NewLabel(dlg)
	_ = modeLabel.SetText("Mode")
	_ = layout.SetRange(modeLabel, walk.Rectangle{X: 0, Y: 1, Width: 1, Height: 1})

	modeOptions := []string{"DSN", "Driver", "File DSN", "Connection string"}
	modeSelect, _ := walk.NewDropDownBox(dlg)
	_ = modeSelect.SetModel(modeOptions)
	_ = modeSelect.SetCurrentIndex(0)
	_ = layout.SetRange(modeSelect, walk.Rectangle{X: 1, Y: 1, Width: 3, Height: 1})

	dsnLabel, _ := walk.NewLabel(dlg)
	_ = dsnLabel.SetText("DSN")
	_ = layout.SetRange(dsnLabel, walk.Rectangle{X: 0, Y: 2, Width: 1, Height: 1})

	dsnEntry, _ := walk.NewLineEdit(dlg)
	_ = layout.SetRange(dsnEntry, walk.Rectangle{X: 1, Y: 2, Width: 3, Height: 1})

	driverLabel, _ := walk.NewLabel(dlg)
	_ = driverLabel.SetText("Driver")
	_ = layout.SetRange(driverLabel, walk.Rectangle{X: 0, Y: 3, Width: 1, Height: 1})

	driverEntry, _ := walk.NewLineEdit(dlg)
	_ = layout.SetRange(driverEntry, walk.Rectangle{X: 1, Y: 3, Width: 3, Height: 1})

	fileLabel, _ := walk.NewLabel(dlg)
	_ = fileLabel.SetText("File DSN")
	_ = layout.SetRange(fileLabel, walk.Rectangle{X: 0, Y: 4, Width: 1, Height: 1})

	fileEntry, _ := walk.NewLineEdit(dlg)
	_ = layout.SetRange(fileEntry, walk.Rectangle{X: 1, Y: 4, Width: 3, Height: 1})

	connLabel, _ := walk.NewLabel(dlg)
	_ = connLabel.SetText("Connection string")
	_ = layout.SetRange(connLabel, walk.Rectangle{X: 0, Y: 5, Width: 1, Height: 1})

	connEntry, _ := walk.NewTextEdit(dlg)
	_ = layout.SetRange(connEntry, walk.Rectangle{X: 1, Y: 5, Width: 3, Height: 2})

	nameLabel, _ := walk.NewLabel(dlg)
	_ = nameLabel.SetText("Name")
	_ = layout.SetRange(nameLabel, walk.Rectangle{X: 0, Y: 7, Width: 1, Height: 1})

	nameEntry, _ := walk.NewLineEdit(dlg)
	_ = layout.SetRange(nameEntry, walk.Rectangle{X: 1, Y: 7, Width: 3, Height: 1})

	userLabel, _ := walk.NewLabel(dlg)
	_ = userLabel.SetText("Username")
	_ = layout.SetRange(userLabel, walk.Rectangle{X: 0, Y: 8, Width: 1, Height: 1})

	userEntry, _ := walk.NewLineEdit(dlg)
	_ = layout.SetRange(userEntry, walk.Rectangle{X: 1, Y: 8, Width: 3, Height: 1})

	passLabel, _ := walk.NewLabel(dlg)
	_ = passLabel.SetText("Password")
	_ = layout.SetRange(passLabel, walk.Rectangle{X: 0, Y: 9, Width: 1, Height: 1})

	passEntry, _ := walk.NewLineEdit(dlg)
	passEntry.SetPasswordMode(true)
	_ = layout.SetRange(passEntry, walk.Rectangle{X: 1, Y: 9, Width: 3, Height: 1})

	savePasswordCheck, _ := walk.NewCheckBox(dlg)
	_ = savePasswordCheck.SetText("Save password securely")
	_ = layout.SetRange(savePasswordCheck, walk.Rectangle{X: 1, Y: 10, Width: 3, Height: 1})

	finalLabel, _ := walk.NewLabel(dlg)
	_ = finalLabel.SetText("Final connection string")
	_ = layout.SetRange(finalLabel, walk.Rectangle{X: 0, Y: 11, Width: 1, Height: 1})

	finalView, _ := walk.NewTextEdit(dlg)
	_ = finalView.SetReadOnly(true)
	_ = layout.SetRange(finalView, walk.Rectangle{X: 1, Y: 11, Width: 3, Height: 2})

	profileOptions := make([]string, 0, len(u.cfg.Connections))
	profileMap := map[string]config.ConnectionProfile{}
	for _, p := range u.cfg.Connections {
		label := p.Name + " [" + p.Type + "]"
		profileOptions = append(profileOptions, label)
		profileMap[label] = p
	}
	_ = profileSelect.SetModel(profileOptions)
	if len(profileOptions) > 0 {
		_ = profileSelect.SetCurrentIndex(0)
	}

	selectedProfileID := ""

	applyModeVisibility := func() {
		mode := comboSelected(modeSelect, modeOptions)
		dsnLabel.SetVisible(mode == "DSN")
		dsnEntry.SetVisible(mode == "DSN")
		driverLabel.SetVisible(mode == "Driver")
		driverEntry.SetVisible(mode == "Driver")
		fileLabel.SetVisible(mode == "File DSN")
		fileEntry.SetVisible(mode == "File DSN")
		connLabel.SetVisible(mode == "Connection string")
		connEntry.SetVisible(mode == "Connection string")
	}

	profileSelect.CurrentIndexChanged().Attach(func() {
		label := comboSelected(profileSelect, profileOptions)
		p, ok := profileMap[label]
		if !ok {
			return
		}
		selectedProfileID = p.ID
		_ = nameEntry.SetText(p.Name)
		_ = userEntry.SetText(p.Username)
		savePasswordCheck.SetChecked(p.SavePassword)
		_ = passEntry.SetText("")
		_ = dsnEntry.SetText(p.DSN)
		_ = driverEntry.SetText(p.Driver)
		_ = fileEntry.SetText(p.FilePath)
		_ = connEntry.SetText(p.ConnectionString)

		switch p.Type {
		case "driver":
			_ = modeSelect.SetCurrentIndex(indexOf(modeOptions, "Driver"))
		case "file_dsn":
			_ = modeSelect.SetCurrentIndex(indexOf(modeOptions, "File DSN"))
		case "connection_string":
			_ = modeSelect.SetCurrentIndex(indexOf(modeOptions, "Connection string"))
		default:
			_ = modeSelect.SetCurrentIndex(indexOf(modeOptions, "DSN"))
		}
		applyModeVisibility()
	})

	modeSelect.CurrentIndexChanged().Attach(applyModeVisibility)

	buildProfile := func() (config.ConnectionProfile, string, error) {
		name := strings.TrimSpace(nameEntry.Text())
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
			DSN:          strings.TrimSpace(dsnEntry.Text()),
			Driver:       strings.TrimSpace(driverEntry.Text()),
			FilePath:     strings.TrimSpace(fileEntry.Text()),
			Username:     strings.TrimSpace(userEntry.Text()),
			SavePassword: savePasswordCheck.Checked(),
			Options: config.ConnectionOptions{
				LoginTimeoutMs: 30000,
				LimitSyntax:    "TOP",
			},
		}
		password := passEntry.Text()

		switch comboSelected(modeSelect, modeOptions) {
		case "Driver":
			if profile.Driver == "" {
				return profile, password, errors.New("driver is required")
			}
			profile.Type = "driver"
		case "File DSN":
			if profile.FilePath == "" {
				return profile, password, errors.New("file DSN path is required")
			}
			profile.Type = "file_dsn"
		case "Connection string":
			profile.Type = "connection_string"
			profile.ConnectionString = strings.TrimSpace(connEntry.Text())
			if profile.ConnectionString == "" {
				return profile, password, errors.New("connection string is required")
			}
		default:
			profile.Type = "dsn"
			if profile.DSN == "" {
				return profile, password, errors.New("DSN is required")
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

		finalConn, err := odbc.BuildConnectionString(profile, password)
		if err != nil {
			return profile, password, err
		}
		_ = finalView.SetText(odbc.MaskSecrets(finalConn))

		return profile, password, nil
	}

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

	buttonRow, _ := walk.NewComposite(dlg)
	rowLayout := walk.NewHBoxLayout()
	_ = rowLayout.SetMargins(walk.Margins{})
	_ = rowLayout.SetSpacing(8)
	_ = buttonRow.SetLayout(rowLayout)
	_ = layout.SetRange(buttonRow, walk.Rectangle{X: 0, Y: 13, Width: 4, Height: 1})

	testBtn, _ := walk.NewPushButton(buttonRow)
	_ = testBtn.SetText("Test")
	saveBtn, _ := walk.NewPushButton(buttonRow)
	_ = saveBtn.SetText("Save Profile")
	connectBtn, _ := walk.NewPushButton(buttonRow)
	_ = connectBtn.SetText("Connect")
	cancelBtn, _ := walk.NewPushButton(buttonRow)
	_ = cancelBtn.SetText("Cancel")

	testBtn.Clicked().Attach(func() {
		profile, password, buildErr := buildProfile()
		if buildErr != nil {
			showErrorBox(dlg, "Connection Test", buildErr)
			return
		}
		go func() {
			elapsed, testErr := u.manager.TestConnection(profile, password, time.Duration(profile.Options.LoginTimeoutMs)*time.Millisecond)
			dlg.Synchronize(func() {
				if testErr != nil {
					showErrorBox(dlg, "Connection Test", testErr)
					return
				}
				showInfoBox(dlg, "Connection Test", "Success in "+elapsed.String())
			})
		}()
	})

	saveBtn.Clicked().Attach(func() {
		profile, password, buildErr := buildProfile()
		if buildErr != nil {
			showErrorBox(dlg, "Profiles", buildErr)
			return
		}
		if err := saveProfile(profile, password); err != nil {
			showErrorBox(dlg, "Profiles", err)
			return
		}
		showInfoBox(dlg, "Profiles", "Profile saved.")
	})

	connectBtn.Clicked().Attach(func() {
		profile, password, buildErr := buildProfile()
		if buildErr != nil {
			showErrorBox(dlg, "Connect", buildErr)
			return
		}
		go func() {
			_, connectErr := u.manager.Connect(profile, password, time.Duration(profile.Options.LoginTimeoutMs)*time.Millisecond)
			dlg.Synchronize(func() {
				if connectErr != nil {
					showErrorBox(dlg, "Connect", connectErr)
					return
				}
				if saveErr := saveProfile(profile, password); saveErr != nil {
					showErrorBox(dlg, "Connect", saveErr)
					return
				}
				u.refreshSessionSelect()
				u.updateStatus()
				dlg.Accept()
			})
		}()
	})

	cancelBtn.Clicked().Attach(func() {
		dlg.Cancel()
	})

	_ = dlg.SetDefaultButton(connectBtn)
	_ = dlg.SetCancelButton(cancelBtn)

	applyModeVisibility()
	dlg.Run()
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
	showInfoBox(u.mainWindow, "About", report.String())
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

	u.sessionOptions = options
	_ = u.sessionSelect.SetModel(options)
	if selected != "" {
		_ = u.sessionSelect.SetCurrentIndex(indexOf(options, selected))
	} else if len(options) > 0 {
		_ = u.sessionSelect.SetCurrentIndex(0)
	}
}

func (u *applicationUI) updateStatus() {
	session, err := u.manager.ActiveSession()
	if err != nil {
		_ = u.statusLabel.SetText("Disconnected")
		return
	}

	msg := fmt.Sprintf("Active: %s | State: %s", session.Name, session.State)
	if session.Tx != nil {
		msg += " | Transaction: ON"
	}
	if !session.LastQueryAt.IsZero() {
		msg += " | Last Query: " + session.LastQueryAt.Format(time.RFC3339)
	}
	_ = u.statusLabel.SetText(msg)
}

func (u *applicationUI) restoreProfiles() {
	u.refreshSessionSelect()
	u.updateStatus()
}

func (u *applicationUI) promptParameters(count int, callback func([]any)) {
	dlg, err := walk.NewDialog(u.mainWindow)
	if err != nil {
		showErrorBox(u.mainWindow, "Parameters", err)
		return
	}
	_ = dlg.SetTitle("Parameters")
	_ = dlg.SetBounds(walk.Rectangle{Width: 560, Height: 430})

	layout := walk.NewVBoxLayout()
	_ = layout.SetMargins(walk.Margins{8, 8, 8, 8})
	_ = layout.SetSpacing(8)
	if err := dlg.SetLayout(layout); err != nil {
		showErrorBox(u.mainWindow, "Parameters", err)
		return
	}

	scroll, err := walk.NewScrollView(dlg)
	if err != nil {
		showErrorBox(u.mainWindow, "Parameters", err)
		return
	}
	_ = layout.SetStretchFactor(scroll, 1)

	rowsLayout := walk.NewVBoxLayout()
	_ = rowsLayout.SetMargins(walk.Margins{})
	_ = rowsLayout.SetSpacing(6)
	if err := scroll.SetLayout(rowsLayout); err != nil {
		showErrorBox(u.mainWindow, "Parameters", err)
		return
	}

	typeOptions := []string{"Auto", "String", "Int", "Float", "Bool", "Null"}

	type paramInput struct {
		Type  *walk.ComboBox
		Value *walk.LineEdit
	}

	inputs := make([]paramInput, 0, count)

	for i := 0; i < count; i++ {
		row, rowErr := walk.NewComposite(scroll)
		if rowErr != nil {
			showErrorBox(u.mainWindow, "Parameters", rowErr)
			return
		}

		rowLayout := walk.NewHBoxLayout()
		_ = rowLayout.SetMargins(walk.Margins{})
		_ = rowLayout.SetSpacing(6)
		_ = row.SetLayout(rowLayout)

		label, _ := walk.NewLabel(row)
		_ = label.SetText(fmt.Sprintf("Param %d", i+1))
		_ = label.SetMinMaxSize(walk.Size{Width: 72}, walk.Size{Width: 72})

		typ, _ := walk.NewDropDownBox(row)
		_ = typ.SetModel(typeOptions)
		_ = typ.SetCurrentIndex(0)

		value, _ := walk.NewLineEdit(row)
		_ = rowLayout.SetStretchFactor(value, 1)

		inputs = append(inputs, paramInput{Type: typ, Value: value})
	}

	buttons, _ := walk.NewComposite(dlg)
	buttonsLayout := walk.NewHBoxLayout()
	_ = buttonsLayout.SetMargins(walk.Margins{})
	_ = buttonsLayout.SetSpacing(6)
	_ = buttons.SetLayout(buttonsLayout)

	runBtn, _ := walk.NewPushButton(buttons)
	_ = runBtn.SetText("Run")

	cancelBtn, _ := walk.NewPushButton(buttons)
	_ = cancelBtn.SetText("Cancel")

	runBtn.Clicked().Attach(func() {
		values := make([]any, 0, count)
		for _, input := range inputs {
			raw := input.Value.Text()
			switch comboSelected(input.Type, typeOptions) {
			case "Null":
				values = append(values, nil)
			case "String":
				values = append(values, raw)
			case "Int":
				iv, parseErr := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
				if parseErr != nil {
					showErrorBox(dlg, "Parameters", parseErr)
					return
				}
				values = append(values, iv)
			case "Float":
				fv, parseErr := strconv.ParseFloat(strings.TrimSpace(raw), 64)
				if parseErr != nil {
					showErrorBox(dlg, "Parameters", parseErr)
					return
				}
				values = append(values, fv)
			case "Bool":
				bv, parseErr := strconv.ParseBool(strings.TrimSpace(raw))
				if parseErr != nil {
					showErrorBox(dlg, "Parameters", parseErr)
					return
				}
				values = append(values, bv)
			default:
				values = append(values, raw)
			}
		}
		dlg.Accept()
		callback(values)
	})

	cancelBtn.Clicked().Attach(func() {
		dlg.Cancel()
	})

	_ = dlg.SetDefaultButton(runBtn)
	_ = dlg.SetCancelButton(cancelBtn)
	dlg.Run()
}

func (u *applicationUI) promptLine(title, label, initial string) (string, bool) {
	dlg, err := walk.NewDialog(u.mainWindow)
	if err != nil {
		showErrorBox(u.mainWindow, title, err)
		return "", false
	}
	_ = dlg.SetTitle(title)
	_ = dlg.SetBounds(walk.Rectangle{Width: 420, Height: 160})

	layout := walk.NewVBoxLayout()
	_ = layout.SetMargins(walk.Margins{8, 8, 8, 8})
	_ = layout.SetSpacing(8)
	_ = dlg.SetLayout(layout)

	lbl, _ := walk.NewLabel(dlg)
	_ = lbl.SetText(label)

	entry, _ := walk.NewLineEdit(dlg)
	_ = entry.SetText(initial)

	buttons, _ := walk.NewComposite(dlg)
	rowLayout := walk.NewHBoxLayout()
	_ = rowLayout.SetMargins(walk.Margins{})
	_ = rowLayout.SetSpacing(6)
	_ = buttons.SetLayout(rowLayout)

	okBtn, _ := walk.NewPushButton(buttons)
	_ = okBtn.SetText("Save")

	cancelBtn, _ := walk.NewPushButton(buttons)
	_ = cancelBtn.SetText("Cancel")

	okBtn.Clicked().Attach(func() {
		dlg.Accept()
	})
	cancelBtn.Clicked().Attach(func() {
		dlg.Cancel()
	})

	_ = dlg.SetDefaultButton(okBtn)
	_ = dlg.SetCancelButton(cancelBtn)

	if dlg.Run() == walk.DlgCmdOK {
		return entry.Text(), true
	}
	return "", false
}

func (u *applicationUI) runOnMain(fn func()) {
	if fn == nil {
		return
	}
	if u.mainWindow == nil {
		fn()
		return
	}
	u.mainWindow.Synchronize(fn)
}

func selectedText(editor *walk.TextEdit) string {
	if editor == nil {
		return ""
	}
	start, end := editor.TextSelection()
	if end <= start {
		return ""
	}
	runes := []rune(editor.Text())
	if start < 0 || start >= len(runes) || end > len(runes) {
		return ""
	}
	return string(runes[start:end])
}

func comboSelected(combo *walk.ComboBox, options []string) string {
	if combo == nil {
		return ""
	}
	idx := combo.CurrentIndex()
	if idx < 0 || idx >= len(options) {
		return ""
	}
	return options[idx]
}

func indexOf(items []string, target string) int {
	for i := range items {
		if items[i] == target {
			return i
		}
	}
	return -1
}

func showInfoBox(owner walk.Form, title, message string) {
	walk.MsgBox(owner, title, message, walk.MsgBoxOK|walk.MsgBoxIconInformation)
}

func showErrorBox(owner walk.Form, title string, err error) {
	if err == nil {
		return
	}
	walk.MsgBox(owner, title, err.Error(), walk.MsgBoxOK|walk.MsgBoxIconError)
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
