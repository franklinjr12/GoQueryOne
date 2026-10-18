//go:build windows

package ui

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/franklinjr12/GoQueryOne/internal/app"
	"github.com/franklinjr12/GoQueryOne/internal/buildinfo"
	"github.com/franklinjr12/GoQueryOne/internal/config"
	"github.com/franklinjr12/GoQueryOne/internal/core"
	"github.com/franklinjr12/GoQueryOne/internal/export"
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
	nulls   [][]bool
}

// nullDisplayText is how SQL NULL is rendered in the grid (distinct from the string "NULL").
const nullDisplayText = "(null)"

func (m *resultsTableModel) RowCount() int {
	return len(m.rows)
}

// Value returns the cell text. Column 0 is the 1-based row number; data
// columns start at 1. SQL NULL is rendered as nullDisplayText.
func (m *resultsTableModel) Value(row, col int) interface{} {
	if row < 0 || row >= len(m.rows) {
		return ""
	}
	if len(m.columns) == 0 {
		if col == 0 && len(m.rows[row]) > 0 {
			return m.rows[row][0]
		}
		return ""
	}
	if col == 0 {
		return strconv.Itoa(row + 1)
	}
	col--
	if col < 0 || col >= len(m.columns) || col >= len(m.rows[row]) {
		return ""
	}
	if m.isNull(row, col) {
		return nullDisplayText
	}
	return m.rows[row][col]
}

func (m *resultsTableModel) isNull(row, col int) bool {
	return row >= 0 && col >= 0 && row < len(m.nulls) && col < len(m.nulls[row]) && m.nulls[row][col]
}

// StyleCell greys out NULL cells and the row-number column.
func (m *resultsTableModel) StyleCell(style *walk.CellStyle) {
	if len(m.columns) == 0 {
		return
	}
	row, col := style.Row(), style.Col()
	if row < 0 || col < 0 {
		// walk also asks for whole-row styles (col -1); nothing to do there.
		return
	}
	if col == 0 {
		style.TextColor = walk.RGB(120, 120, 120)
		style.BackgroundColor = walk.RGB(243, 243, 243)
		return
	}
	if m.isNull(row, col-1) {
		style.TextColor = walk.RGB(150, 150, 150)
	}
}

func (m *resultsTableModel) SetData(columns []string, rows [][]string, nulls [][]bool) {
	m.columns = append([]string(nil), columns...)
	m.rows = rows
	m.nulls = nulls
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

	actions        *uiActions
	presenter      *app.Presenter
	activityText   string
	executing      bool
	executingSince time.Time
	lastTruncated  bool
	resultsText    *walk.TextEdit
	textView       bool

	editorTabs     *walk.TabWidget
	editorTabIndex map[*walk.TabPage]*sqlEditorTab

	outputTabs     *walk.TabWidget
	resultsTable   *walk.TableView
	resultsModel   *resultsTableModel
	resultsColumns []string
	resultsRows    [][]string
	resultsNulls   [][]bool
	selectedRow    int
	selectedCol    int
	loadMoreButton *walk.PushButton
	lastQuerySQL   string
	lastQueryArgs  []any
	lastMaxRows    int

	schemaSearchEntry *walk.LineEdit
	schemaTree        *walk.TreeView
	schemaSnapshot    odbc.SchemaSnapshot
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
		manager:          newManager(),
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
	// Default size in DIPs (scaled by walk); the saved bounds are applied in
	// pixels after creation by placeMainWindow.
	mainWindow := declarative.MainWindow{
		AssignTo: &u.mainWindow,
		Title:    "GoQueryOne",
		Bounds:   declarative.Rectangle{Width: 1260, Height: 760},
		MinSize:  declarative.Size{Width: 1024, Height: 640},
		Layout:   declarative.VBox{MarginsZero: true, Spacing: 6},
		Visible:  false,
	}
	if err := mainWindow.Create(); err != nil {
		return err
	}

	if icon, err := walk.NewIconFromResource("APP"); err == nil {
		_ = u.mainWindow.SetIcon(icon)
	}
	if err := u.buildActions(); err != nil {
		return err
	}
	if err := u.build(); err != nil {
		return err
	}
	u.restoreProfiles()
	u.applyHistorySetting()
	u.refreshState()
	u.placeMainWindow()
	u.showMainWindow()
	if tab := u.currentEditorTab(); tab != nil {
		_ = tab.Editor.SetFocus()
	}
	u.mainWindow.Run()
	return nil
}

func (u *applicationUI) build() error {
	layout, ok := u.mainWindow.Layout().(*walk.BoxLayout)
	if !ok {
		return errors.New("main window layout initialization failed")
	}
	_ = layout.SetMargins(walk.Margins{HNear: 6, VNear: 6, HFar: 6, VFar: 6})
	_ = layout.SetSpacing(6)

	topBar, err := u.buildTopBar()
	if err != nil {
		return err
	}
	_ = topBar.SetMinMaxSize(walk.Size{Height: 26}, walk.Size{Height: 26})

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

	_ = layout.SetStretchFactor(mainSplit, 1)

	u.mainWindow.Closing().Attach(func(canceled *bool, reason walk.CloseReason) {
		if !u.confirmDiscardAll() {
			*canceled = true
			return
		}
		u.saveMainWindowPlacement()
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

	sessionLabel, err := walk.NewLabel(topBar)
	if err != nil {
		return nil, err
	}
	_ = sessionLabel.SetText("Active connection:")

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
		u.refreshState()
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

	u.editorTabs, err = walk.NewTabWidget(panel)
	if err != nil {
		return nil, err
	}
	u.editorTabs.CurrentIndexChanged().Attach(u.refreshState)
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
	if font := monoFont(app.EditorFontSize(*u.cfg)); font != nil {
		editor.SetFont(font)
	}
	editor.KeyUp().Attach(func(walk.Key) { u.refreshState() })
	editor.MouseUp().Attach(func(int, int, walk.MouseButton) { u.refreshState() })

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
	u.refreshState()
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
	pages := u.editorTabs.Pages()
	idx := u.editorTabs.CurrentIndex()
	if idx < 0 || idx >= pages.Len() {
		return
	}
	page := pages.At(idx)
	tab := u.editorTabIndex[page]
	if tab == nil {
		return
	}
	if !u.confirmDiscard(tab) {
		return
	}

	if pages.Len() == 1 {
		tab.suppressDirty = true
		_ = tab.Editor.SetText("")
		tab.suppressDirty = false
		tab.Dirty = false
		u.refreshTabTitle(tab)
		return
	}

	nextIndex := idx - 1
	if nextIndex < 0 {
		nextIndex = 1
	}
	_ = u.editorTabs.SetCurrentIndex(nextIndex)

	if err := pages.Remove(page); err != nil {
		showErrorBox(u.mainWindow, "Editor", err)
		return
	}
	delete(u.editorTabIndex, page)
	page.Dispose()
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

	refreshBtn, err := walk.NewPushButton(buttonRow)
	if err != nil {
		return nil, err
	}
	_ = refreshBtn.SetText("Refresh schema (F8)")
	refreshBtn.Clicked().Attach(func() {
		u.loadSchema(true)
	})

	if err := u.buildSchemaTree(topSection, topLayout); err != nil {
		return nil, err
	}

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
		_ = walk.Clipboard().SetText(u.sqlName(*selected))
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
	u.outputTabs = tabs
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

	loadMoreBar, err := walk.NewComposite(resultsPage)
	if err != nil {
		return nil, err
	}
	loadMoreLayout := walk.NewHBoxLayout()
	_ = loadMoreLayout.SetMargins(walk.Margins{})
	if err := loadMoreBar.SetLayout(loadMoreLayout); err != nil {
		return nil, err
	}
	u.loadMoreButton, err = walk.NewPushButton(loadMoreBar)
	if err != nil {
		return nil, err
	}
	_ = u.loadMoreButton.SetText("Result truncated - Load more rows")
	u.loadMoreButton.Clicked().Attach(func() {
		u.loadMoreRows()
	})
	_, _ = walk.NewHSpacer(loadMoreBar)
	u.loadMoreButton.SetVisible(false)

	u.resultsTable, err = walk.NewTableView(resultsPage)
	if err != nil {
		return nil, err
	}
	if err := u.resultsTable.SetModel(u.resultsModel); err != nil {
		return nil, err
	}
	_ = u.resultsTable.SetLastColumnStretched(false)
	_ = u.resultsTable.SetMultiSelection(true)
	u.resultsTable.SetAlternatingRowBG(true)
	u.resultsTable.SetCellStyler(u.resultsModel)
	u.resultsTable.CurrentIndexChanged().Attach(func() {
		u.selectedRow = u.resultsTable.CurrentIndex()
		u.selectedCol = 0
	})
	u.resultsTable.SelectedIndexesChanged().Attach(u.refreshState)
	u.resultsTable.KeyDown().Attach(func(key walk.Key) {
		if key == walk.KeyC && walk.ModifiersDown() == walk.ModControl {
			u.copySelectedRows(false)
		}
		if key == walk.KeyA && walk.ModifiersDown() == walk.ModControl {
			_ = u.resultsTable.SetSelectedIndexes(allIndexes(len(u.resultsRows)))
		}
	})
	_ = resultsLayout.SetStretchFactor(u.resultsTable, 1)

	u.resultsText, err = walk.NewTextEdit(resultsPage)
	if err != nil {
		return nil, err
	}
	_ = u.resultsText.SetReadOnly(true)
	if font := monoFont(9); font != nil {
		u.resultsText.SetFont(font)
	}
	u.resultsText.SetVisible(false)
	_ = resultsLayout.SetStretchFactor(u.resultsText, 1)
	u.preventSelectAllOnFocus(u.resultsText)

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
	u.preventSelectAllOnFocus(entry)
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
	numCol := walk.NewTableViewColumn()
	_ = numCol.SetTitle("#")
	_ = numCol.SetAlignment(walk.AlignFar)
	numCol.SetWidth(columnWidthPx(len(strconv.Itoa(len(u.resultsRows))) + 1))
	_ = cols.Add(numCol)
	for i, name := range u.resultsColumns {
		col := walk.NewTableViewColumn()
		_ = col.SetTitle(name)
		if isNumericColumn(u.resultsRows, u.resultsNulls, i) {
			_ = col.SetAlignment(walk.AlignFar)
		}
		col.SetWidth(columnWidthPx(columnChars(name, u.resultsRows, i)))
		_ = cols.Add(col)
	}
}

const autoWidthSampleRows = 200

// columnChars returns the widest display length (runes) of the header and
// the first autoWidthSampleRows values of column col.
func columnChars(header string, rows [][]string, col int) int {
	n := utf8.RuneCountInString(header)
	if n < len(nullDisplayText) {
		n = len(nullDisplayText)
	}
	for r := 0; r < len(rows) && r < autoWidthSampleRows; r++ {
		if col < len(rows[r]) {
			if l := utf8.RuneCountInString(rows[r][col]); l > n {
				n = l
			}
		}
	}
	return n
}

// columnWidthPx converts a character count to a capped pixel width.
func columnWidthPx(chars int) int {
	w := chars*7 + 18
	if w < 40 {
		w = 40
	}
	if w > 400 {
		w = 400
	}
	return w
}

// isNumericColumn reports whether every non-NULL sampled value parses as a number.
func isNumericColumn(rows [][]string, nulls [][]bool, col int) bool {
	seen := false
	for r := 0; r < len(rows) && r < autoWidthSampleRows; r++ {
		if col >= len(rows[r]) {
			continue
		}
		if r < len(nulls) && col < len(nulls[r]) && nulls[r][col] {
			continue
		}
		v := strings.TrimSpace(rows[r][col])
		if v == "" {
			return false
		}
		if _, err := strconv.ParseFloat(v, 64); err != nil {
			return false
		}
		seen = true
	}
	return seen
}

func allIndexes(n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = i
	}
	return out
}

// preventSelectAllOnFocus stops read-only boxes from highlighting their
// whole content when they receive focus.
func (u *applicationUI) preventSelectAllOnFocus(te *walk.TextEdit) {
	te.FocusedChanged().Attach(func() {
		if te.Focused() {
			start, end := te.TextSelection()
			if end-start == te.TextLength() && end > 0 {
				te.SetTextSelection(end, end)
			}
		}
	})
}

// copySelectedRows copies the selected grid rows as Excel-ready TSV.
func (u *applicationUI) copySelectedRows(withHeaders bool) {
	if len(u.resultsColumns) == 0 || u.resultsTable == nil {
		return
	}
	idx := u.resultsTable.SelectedIndexes()
	if len(idx) == 0 {
		if cur := u.resultsTable.CurrentIndex(); cur >= 0 {
			idx = []int{cur}
		} else {
			return
		}
	}
	_ = walk.Clipboard().SetText(export.TSVString(export.SelectRows(u.currentTable(), idx), withHeaders))
	u.setActivity(fmt.Sprintf("Copied %d row(s)", len(idx)))
	u.activityText = ""
}

// toggleTextView switches the Results tab between the grid and an aligned
// plain-text rendering (F9, like ODBC Query Tool).
func (u *applicationUI) toggleTextView() {
	u.textView = !u.textView
	u.renderResultView()
	if u.outputTabs != nil {
		_ = u.outputTabs.SetCurrentIndex(0)
	}
}

func (u *applicationUI) renderResultView() {
	if u.resultsTable == nil || u.resultsText == nil {
		return
	}
	if u.textView {
		_ = u.resultsText.SetText(strings.ReplaceAll(export.TextString(u.currentTable(), export.TextOptions{}), "\n", "\r\n"))
	}
	u.resultsText.SetVisible(u.textView)
	u.resultsTable.SetVisible(!u.textView)
}

// saveTab writes the tab to its file; asks for a path when it has none or
// when asNew (Save As). Returns false if the user canceled or saving failed.
func (u *applicationUI) saveTab(tab *sqlEditorTab, asNew bool) bool {
	if tab == nil {
		return false
	}
	path := tab.FilePath
	if asNew || strings.TrimSpace(path) == "" {
		dlg := walk.FileDialog{
			Title:       "Save SQL",
			FilePath:    tab.FilePath,
			Filter:      "SQL Files (*.sql;*.txt)|*.sql;*.txt|All Files (*.*)|*.*",
			FilterIndex: 1,
		}
		if strings.TrimSpace(dlg.FilePath) == "" {
			dlg.FilePath = strings.TrimSuffix(tab.Title, " *") + ".sql"
		}
		accepted, err := dlg.ShowSave(u.mainWindow)
		if err != nil {
			showErrorBox(u.mainWindow, "Save SQL", err)
			return false
		}
		if !accepted {
			return false
		}
		path = dlg.FilePath
		if filepath.Ext(path) == "" {
			path += ".sql"
		}
	}
	if err := os.WriteFile(path, []byte(tab.Editor.Text()), 0o644); err != nil {
		showErrorBox(u.mainWindow, "Save SQL", err)
		return false
	}
	tab.FilePath = path
	tab.Title = filepath.Base(path)
	tab.Dirty = false
	u.refreshTabTitle(tab)
	return true
}

// confirmDiscard asks Save / Don't save / Cancel for a dirty tab. Returns
// true when it is OK to close the tab.
func (u *applicationUI) confirmDiscard(tab *sqlEditorTab) bool {
	if tab == nil || !tab.Dirty {
		return true
	}
	if u.editorTabs != nil && tab.Page != nil {
		if idx := u.editorTabs.Pages().Index(tab.Page); idx >= 0 {
			_ = u.editorTabs.SetCurrentIndex(idx)
		}
	}
	switch walk.MsgBox(u.mainWindow, "Unsaved changes", "Save changes to \""+tab.Title+"\"?", walk.MsgBoxYesNoCancel|walk.MsgBoxIconQuestion) {
	case walk.DlgCmdYes:
		return u.saveTab(tab, false)
	case walk.DlgCmdNo:
		return true
	default:
		return false
	}
}

func (u *applicationUI) confirmDiscardAll() bool {
	if u.editorTabs == nil {
		return true
	}
	pages := u.editorTabs.Pages()
	for i := 0; i < pages.Len(); i++ {
		if !u.confirmDiscard(u.editorTabIndex[pages.At(i)]) {
			return false
		}
	}
	return true
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
		// No selection: run the statement under the caret (like ODBC Query Tool)
		// and select it so the user sees exactly what was executed.
		text := tab.Editor.Text()
		caret, _ := tab.Editor.TextSelection()
		stmt, start, end, ok := core.StatementAt(text, core.UTF16OffsetToByteOffset(text, caret))
		if ok {
			sqlText = strings.TrimSpace(stmt)
			tab.Editor.SetTextSelection(core.ByteOffsetToUTF16Offset(text, start), core.ByteOffsetToUTF16Offset(text, end))
		}
	}
	if sqlText == "" {
		showInfoBox(u.mainWindow, "Run", "No SQL to execute.")
		return
	}

	paramCount := odbc.CountPositionalParams(sqlText)
	run := func(params []any) {
		u.beginExecuting("Executing…")
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
				} else if u.outputTabs != nil {
					// Auto-switch: rows -> Results, commands -> Messages.
					if res.HasRows {
						_ = u.outputTabs.SetCurrentIndex(0)
					} else {
						_ = u.outputTabs.SetCurrentIndex(1)
					}
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

	u.beginExecuting("Executing script…")
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
				b.WriteString(fmt.Sprintf("%d. %s (%s)\n", i+1, status, core.FormatDuration(item.ExecutionTime)))
				switch {
				case item.ErrorMessage != "":
					b.WriteString("   " + odbc.MaskSecrets(item.ErrorMessage) + "\n")
				case item.HasRows:
					b.WriteString(fmt.Sprintf("   rows: %d\n", item.ResultSet.RowCount))
				case item.RowsAffected >= 0:
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
	if res.ErrorMessage != "" {
		// Failures are reported in the Errors tab; never print a fake "completed" line.
		o := outcomeOf(res)
		o.Err = odbc.MaskSecrets(o.Err)
		u.appendMessage(app.MessageFor(*o))
		return
	}
	if res.HasRows {
		u.resultsColumns = make([]string, len(res.ResultSet.Columns))
		for i, c := range res.ResultSet.Columns {
			u.resultsColumns[i] = c.Name
		}
		u.resultsRows = res.ResultSet.Rows
		u.resultsNulls = res.ResultSet.Nulls
		u.rebuildResultsColumns()
		u.resultsModel.SetData(u.resultsColumns, u.resultsRows, u.resultsNulls)
		u.lastTruncated = res.ResultSet.Truncated
		u.renderResultView()
		u.appendMessage(app.MessageFor(*outcomeOf(res)))
		if u.outputEnabled {
			u.appendMessage(export.TextString(u.currentTable(), export.TextOptions{}))
		}
		return
	}

	u.resultsColumns = []string{}
	u.resultsRows = [][]string{}
	u.resultsNulls = nil
	u.lastTruncated = false
	u.rebuildResultsColumns()
	u.resultsModel.SetData(u.resultsColumns, u.resultsRows, u.resultsNulls)
	u.renderResultView()
	u.appendMessage(app.MessageFor(*outcomeOf(res)))
}

func (u *applicationUI) showDiagnostic(err error, res odbc.StatementResult) {
	var b strings.Builder
	b.WriteString("Error: ")
	b.WriteString(err.Error())
	b.WriteString("\r\n")
	for _, rec := range res.Diagnostics {
		if rec.State != "" {
			b.WriteString("SQLSTATE=" + rec.State + " ")
		}
		if rec.NativeError != 0 {
			b.WriteString(fmt.Sprintf("Native=%d ", rec.NativeError))
		}
		b.WriteString(rec.Message)
		b.WriteString("\r\n")
	}
	_ = u.errorsEntry.SetText(strings.TrimSpace(odbc.MaskSecrets(b.String())))
	u.showErrorsTab()
}

func (u *applicationUI) showErrorsTab() {
	if u.outputTabs == nil {
		return
	}
	_ = u.outputTabs.SetCurrentIndex(2)
}

func (u *applicationUI) updateHistory() {
	session, err := u.manager.ActiveSession()
	if err != nil {
		_ = u.historyEntry.SetText("")
		return
	}
	var b strings.Builder
	for _, item := range session.History {
		b.WriteString(fmt.Sprintf("%s [%s] %s\r\n%s\r\n", item.When.Format("2006-01-02 15:04:05"), item.Status, core.FormatDuration(item.Duration), item.SQL))
	}
	_ = u.historyEntry.SetText(strings.TrimSpace(b.String()))
}

func (u *applicationUI) appendMessage(msg string) {
	if strings.TrimSpace(msg) == "" {
		return
	}
	newText := strings.TrimSpace(u.messagesEntry.Text() + "\r\n" + msg)
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

	u.setActivity("Loading schema…")
	search := strings.TrimSpace(u.schemaSearchEntry.Text())

	go func() {
		var snapshot odbc.SchemaSnapshot
		var loadErr error
		if forceRefresh {
			snapshot, loadErr = u.manager.RefreshSchema(session.ID, search)
		} else {
			snapshot, loadErr = u.manager.LoadSchema(session.ID, search)
		}
		_, _ = u.manager.DBInfo(session.ID) // SQLGetInfo: DBMS/driver for About and the limit syntax
		u.runOnMain(func() {
			if loadErr != nil {
				showErrorBox(u.mainWindow, "Schema", loadErr)
				u.updateStatus()
				return
			}
			u.setSchemaSnapshot(snapshot)
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
	u.setSchemaSnapshot(snapshot)
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
			detailsText := strings.ReplaceAll(strings.TrimSpace(b.String()), "\n", "\r\n")
			_ = u.schemaDetails.SetText(detailsText)
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

	name := u.sqlName(*selected)
	query := "SELECT * FROM " + name + ";"
	switch mode {
	case "count":
		query = "SELECT COUNT(*) AS row_count FROM " + name + ";"
	case "top":
		query = core.SelectTop(u.limitSyntax(), name, 100)
	}

	// Append instead of replacing the editor contents (the old behaviour threw
	// away whatever the user had typed), then select the new statement.
	text, start, end := core.AppendStatement(tab.Editor.Text(), query)
	tab.suppressDirty = true
	_ = tab.Editor.SetText(text)
	tab.suppressDirty = false
	tab.Editor.SetTextSelection(core.ByteOffsetToUTF16Offset(text, start), core.ByteOffsetToUTF16Offset(text, end))
	_ = tab.Editor.SetFocus()
	tab.Dirty = true
	u.refreshTabTitle(tab)
	u.refreshState()
}

// limitSyntax returns the row-limit syntax for the active connection: an
// explicit LIMIT/FETCH/FIRST from the profile wins; otherwise (empty, "auto",
// or the old hard-coded "TOP" default) it is guessed from the driver name,
// the connection string and the DSN's registered driver.
func (u *applicationUI) limitSyntax() string {
	session, err := u.manager.ActiveSession()
	if err != nil {
		return core.LimitTop
	}
	p := session.Profile
	if s := core.NormalizeLimitSyntax(p.Options.LimitSyntax); s != "" && s != core.LimitTop {
		return s
	}
	var hints []string
	if info, infoErr := u.manager.DBInfo(session.ID); infoErr == nil {
		hints = append(hints, info.DBMSName, info.DriverName) // SQLGetInfo: the most reliable hint
	}
	hints = append(hints, p.Driver, p.ConnectionString)
	if p.DSN != "" {
		hints = append(hints, dsnDriver(p.DSN))
		hints = append(hints, p.DSN)
	}
	return core.DetectLimitSyntax(hints...)
}

func (u *applicationUI) loadMoreRows() {
	session, err := u.manager.ActiveSession()
	if err != nil {
		return
	}
	if strings.TrimSpace(u.lastQuerySQL) == "" {
		return
	}
	if !odbc.IsQueryStatement(u.lastQuerySQL) {
		// Never re-run DML/DDL just to page results.
		showInfoBox(u.mainWindow, "Load More", "Load More is only available for queries that return rows.")
		return
	}

	u.lastMaxRows += u.cfg.Query.FetchPageSize
	u.appendMessage("Query re-executed to load more rows (max " + strconv.Itoa(u.lastMaxRows) + ").")
	u.beginExecuting("Loading more rows…")

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
	var text string
	if delimiter == "," {
		var b strings.Builder
		_ = export.WriteCSV(&b, u.currentTable(), export.CSVOptions{Header: true, CRLF: true})
		text = b.String()
	} else {
		text = export.TSVString(u.currentTable(), true)
	}
	_ = walk.Clipboard().SetText(text)
}

// currentTable converts the displayed result set into the export model
// (NULLs stay distinct from empty strings and from the text "NULL").
func (u *applicationUI) currentTable() export.Table {
	return export.Table{Columns: u.resultsColumns, Rows: u.resultsRows, Nulls: u.resultsNulls}
}

func (u *applicationUI) exportResults() {
	if len(u.resultsColumns) == 0 {
		showInfoBox(u.mainWindow, "Export", "No result set loaded.")
		return
	}

	dlg := walk.FileDialog{
		Title:       "Export Results",
		FilePath:    "result.csv",
		Filter:      "CSV (*.csv)|*.csv|TSV (*.tsv)|*.tsv|JSON (*.json)|*.json|Markdown (*.md)|*.md|Text (*.txt)|*.txt",
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

	format, ok := export.ByExtension(dlg.FilePath)
	if !ok {
		format = export.CSV
		if !strings.Contains(filepath.Base(dlg.FilePath), ".") {
			dlg.FilePath += ".csv"
		}
	}
	file, err := os.Create(dlg.FilePath)
	if err != nil {
		showErrorBox(u.mainWindow, "Export", err)
		return
	}
	writeErr := export.Write(file, u.currentTable(), format)
	closeErr := file.Close()
	if writeErr == nil {
		writeErr = closeErr
	}
	if writeErr != nil {
		showErrorBox(u.mainWindow, "Export", writeErr)
		return
	}

	u.appendMessage("Export completed: " + dlg.FilePath)
}

func (u *applicationUI) showConnectionWindow() {
	dlg, err := walk.NewDialog(u.mainWindow)
	if err != nil {
		showErrorBox(u.mainWindow, "Connection", err)
		return
	}
	// The bitness decides which ODBC drivers/DSNs are visible.
	_ = dlg.SetTitle("Connection - " + buildinfo.Arch() + " build: uses " + bitnessWord() + " ODBC drivers and DSNs")
	_ = dlg.SetMinMaxSize(walk.Size{Width: 980, Height: 460}, walk.Size{})

	layout := walk.NewGridLayout()
	_ = layout.SetMargins(walk.Margins{HNear: 10, VNear: 10, HFar: 10, VFar: 10})
	_ = layout.SetSpacing(8)
	_ = layout.SetColumnStretchFactor(1, 1)
	_ = layout.SetColumnStretchFactor(2, 1)
	_ = layout.SetColumnStretchFactor(3, 1)
	if err := dlg.SetLayout(layout); err != nil {
		showErrorBox(u.mainWindow, "Connection", err)
		return
	}

	profileLabel, _ := walk.NewLabel(dlg)
	_ = profileLabel.SetText("Profile")
	_ = layout.SetRange(profileLabel, walk.Rectangle{X: 0, Y: 0, Width: 1, Height: 1})

	profileSelect, _ := walk.NewDropDownBox(dlg)
	_ = layout.SetRange(profileSelect, walk.Rectangle{X: 1, Y: 0, Width: 2, Height: 1})

	profileButtons, _ := walk.NewComposite(dlg)
	pbLayout := walk.NewHBoxLayout()
	_ = pbLayout.SetMargins(walk.Margins{})
	_ = pbLayout.SetSpacing(4)
	_ = profileButtons.SetLayout(pbLayout)
	_ = layout.SetRange(profileButtons, walk.Rectangle{X: 3, Y: 0, Width: 1, Height: 1})
	newProfileBtn, _ := walk.NewPushButton(profileButtons)
	_ = newProfileBtn.SetText("New")
	dupProfileBtn, _ := walk.NewPushButton(profileButtons)
	_ = dupProfileBtn.SetText("Duplicate")
	delProfileBtn, _ := walk.NewPushButton(profileButtons)
	_ = delProfileBtn.SetText("Delete")

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

	// Editable combo boxes: pick a DSN / driver installed for this exe's
	// bitness (SQLDataSources / SQLDrivers) or type one.
	dsnEntry, _ := walk.NewComboBox(dlg)
	_ = dsnEntry.SetModel(dsnNames())
	_ = layout.SetRange(dsnEntry, walk.Rectangle{X: 1, Y: 2, Width: 3, Height: 1})

	driverLabel, _ := walk.NewLabel(dlg)
	_ = driverLabel.SetText("Driver")
	_ = layout.SetRange(driverLabel, walk.Rectangle{X: 0, Y: 3, Width: 1, Height: 1})

	driverEntry, _ := walk.NewComboBox(dlg)
	_ = driverEntry.SetModel(driverNames())
	_ = layout.SetRange(driverEntry, walk.Rectangle{X: 1, Y: 3, Width: 3, Height: 1})

	fileLabel, _ := walk.NewLabel(dlg)
	_ = fileLabel.SetText("File DSN")
	_ = layout.SetRange(fileLabel, walk.Rectangle{X: 0, Y: 4, Width: 1, Height: 1})

	fileEntry, _ := walk.NewLineEdit(dlg)
	_ = layout.SetRange(fileEntry, walk.Rectangle{X: 1, Y: 4, Width: 2, Height: 1})
	browseBtn, _ := walk.NewPushButton(dlg)
	_ = browseBtn.SetText("Browse…")
	_ = layout.SetRange(browseBtn, walk.Rectangle{X: 3, Y: 4, Width: 1, Height: 1})
	browseBtn.Clicked().Attach(func() {
		fd := walk.FileDialog{Title: "File DSN", Filter: "File DSN (*.dsn)|*.dsn|All files (*.*)|*.*", FilePath: fileEntry.Text()}
		if ok, _ := fd.ShowOpen(dlg); ok {
			_ = fileEntry.SetText(fd.FilePath)
		}
	})

	connLabel, _ := walk.NewLabel(dlg)
	_ = connLabel.SetText("Connection string")
	_ = layout.SetRange(connLabel, walk.Rectangle{X: 0, Y: 5, Width: 1, Height: 1})

	connEntry, _ := walk.NewTextEdit(dlg)
	_ = layout.SetRange(connEntry, walk.Rectangle{X: 1, Y: 5, Width: 3, Height: 2})

	// Templates fill the connection string with <placeholders> to replace.
	templateNames := app.ConnTemplateNames()
	templateSelect, _ := walk.NewDropDownBox(dlg)
	_ = templateSelect.SetModel(templateNames)
	_ = templateSelect.SetCurrentIndex(0)
	_ = layout.SetRange(templateSelect, walk.Rectangle{X: 0, Y: 6, Width: 1, Height: 1})
	templateSelect.CurrentIndexChanged().Attach(func() {
		i := templateSelect.CurrentIndex()
		if i <= 0 || i > len(app.ConnTemplates) {
			return
		}
		_ = connEntry.SetText(app.ConnTemplates[i-1].Template)
		_ = templateSelect.SetCurrentIndex(0)
		_ = connEntry.SetFocus()
	})

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

	var profileOptions []string
	profileMap := map[string]config.ConnectionProfile{}
	selectedProfileID := ""
	// loadProfileList rebuilds the profile picker from the config.
	loadProfileList := func() {
		profileOptions = make([]string, 0, len(u.cfg.Connections))
		profileMap = map[string]config.ConnectionProfile{}
		for _, p := range u.cfg.Connections {
			label := app.ProfileLabel(p)
			profileOptions = append(profileOptions, label)
			profileMap[label] = p
		}
		_ = profileSelect.SetModel(profileOptions)
	}
	loadProfileList()

	applyModeVisibility := func() {
		mode := comboSelected(modeSelect, modeOptions)
		dsnLabel.SetVisible(mode == "DSN")
		dsnEntry.SetVisible(mode == "DSN")
		driverLabel.SetVisible(mode == "Driver")
		driverEntry.SetVisible(mode == "Driver")
		fileLabel.SetVisible(mode == "File DSN")
		fileEntry.SetVisible(mode == "File DSN")
		browseBtn.SetVisible(mode == "File DSN")
		// Driver mode reuses the text box for extra attributes.
		connLabel.SetVisible(mode == "Connection string" || mode == "Driver")
		connEntry.SetVisible(mode == "Connection string" || mode == "Driver")
		templateSelect.SetVisible(mode == "Connection string")
		if mode == "Driver" {
			_ = connLabel.SetText("Attributes (Server=…;Database=…)")
		} else {
			_ = connLabel.SetText("Connection string")
		}
	}

	fillFromProfile := func() {
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
	}
	profileSelect.CurrentIndexChanged().Attach(fillFromProfile)

	// selectProfile reloads the list and selects the profile with id (or
	// nothing when id is not found).
	selectProfile := func(id string) {
		loadProfileList()
		selectedProfileID = ""
		for i, label := range profileOptions {
			if profileMap[label].ID == id {
				_ = profileSelect.SetCurrentIndex(i)
				fillFromProfile()
				return
			}
		}
		_ = profileSelect.SetCurrentIndex(-1)
	}
	clearFields := func() {
		selectedProfileID = ""
		_ = profileSelect.SetCurrentIndex(-1)
		for _, le := range []*walk.LineEdit{nameEntry, userEntry, passEntry, fileEntry} {
			_ = le.SetText("")
		}
		_ = dsnEntry.SetText("")
		_ = driverEntry.SetText("")
		_ = connEntry.SetText("")
		_ = finalView.SetText("")
		savePasswordCheck.SetChecked(false)
		_ = nameEntry.SetFocus()
	}
	newProfileBtn.Clicked().Attach(clearFields)
	dupProfileBtn.Clicked().Attach(func() {
		p, ok := u.cfg.ConnectionByID(selectedProfileID)
		if !ok {
			showInfoBox(dlg, "Profiles", "Select a saved profile to duplicate.")
			return
		}
		cp := app.DuplicateProfile(p, u.cfg.Connections, time.Now())
		u.cfg.UpsertConnection(cp)
		if err := config.SaveConfig(u.cfg, u.configPath); err != nil {
			showErrorBox(dlg, "Profiles", err)
		}
		selectProfile(cp.ID)
	})
	delProfileBtn.Clicked().Attach(func() {
		p, ok := u.cfg.ConnectionByID(selectedProfileID)
		if !ok {
			showInfoBox(dlg, "Profiles", "Select a saved profile to delete.")
			return
		}
		if walk.MsgBox(dlg, "Delete profile", "Delete the profile \""+p.Name+"\"?\nIts saved password is removed too.", walk.MsgBoxYesNo|walk.MsgBoxIconQuestion) != walk.DlgCmdYes {
			return
		}
		if p.CredentialRef != "" && u.secrets != nil {
			_ = u.secrets.Delete(p.CredentialRef)
		}
		u.cfg.RemoveConnection(p.ID)
		if err := config.SaveConfig(u.cfg, u.configPath); err != nil {
			showErrorBox(dlg, "Profiles", err)
		}
		loadProfileList()
		clearFields()
	})

	modeSelect.CurrentIndexChanged().Attach(applyModeVisibility)
	// Preselect the first saved profile after the handler is attached so its
	// fields are actually filled in (previously the dialog opened with the
	// profile selected but empty fields).
	if len(profileOptions) > 0 {
		_ = profileSelect.SetCurrentIndex(0)
		fillFromProfile()
	}

	buildProfile := func() (config.ConnectionProfile, string, error) {
		name := strings.TrimSpace(nameEntry.Text())
		if name == "" {
			name = "Connection " + time.Now().Format("20060102150405")
		}

		profileID := selectedProfileID
		if profileID == "" {
			profileID = app.NewProfileID(name, time.Now())
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
				LimitSyntax:    "auto",
			},
		}
		password := passEntry.Text()

		switch comboSelected(modeSelect, modeOptions) {
		case "Driver":
			if profile.Driver == "" {
				return profile, password, errors.New("driver is required")
			}
			profile.Type = "driver"
			profile.ConnectionString = strings.TrimSpace(connEntry.Text())
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

		if missing := core.UnfilledPlaceholders(profile.ConnectionString); len(missing) > 0 && (profile.Type == "connection_string" || profile.Type == "driver") {
			return profile, password, errors.New("replace the template placeholders: " + strings.Join(missing, ", "))
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

	// Inline result of Test / Save (no message box to dismiss).
	resultLabel, _ := walk.NewLabel(dlg)
	_ = layout.SetRange(resultLabel, walk.Rectangle{X: 0, Y: 14, Width: 4, Height: 1})
	setResult := func(ok bool, text string) {
		if ok {
			resultLabel.SetTextColor(walk.RGB(0, 110, 0))
		} else {
			resultLabel.SetTextColor(walk.RGB(180, 0, 0))
		}
		_ = resultLabel.SetText(text)
	}

	testBtn, _ := walk.NewPushButton(buttonRow)
	_ = testBtn.SetText("&Test")
	saveBtn, _ := walk.NewPushButton(buttonRow)
	_ = saveBtn.SetText("Save Profile")
	connectBtn, _ := walk.NewPushButton(buttonRow)
	_ = connectBtn.SetText("&Connect")
	cancelBtn, _ := walk.NewPushButton(buttonRow)
	_ = cancelBtn.SetText("Cancel")

	testing := false
	testBtn.Clicked().Attach(func() {
		if testing {
			return
		}
		profile, password, buildErr := buildProfile()
		if buildErr != nil {
			setResult(false, "Test: "+buildErr.Error())
			return
		}
		testing = true
		testBtn.SetEnabled(false)
		setResult(true, "Testing connection…")
		go func() {
			elapsed, testErr := u.manager.TestConnection(profile, password, time.Duration(profile.Options.LoginTimeoutMs)*time.Millisecond)
			dlg.Synchronize(func() {
				testing = false
				testBtn.SetEnabled(true)
				if testErr != nil {
					setResult(false, "Test failed: "+odbc.MaskSecrets(testErr.Error()))
					return
				}
				setResult(true, "Test succeeded in "+elapsed.Round(time.Millisecond).String())
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
			setResult(false, "Not saved: "+err.Error())
			return
		}
		// Keep editing the saved profile (a second Save updates it instead
		// of creating another copy).
		selectProfile(profile.ID)
		setResult(true, "Profile \""+profile.Name+"\" saved.")
	})

	connecting := false
	connectBtn.Clicked().Attach(func() {
		// Connect runs in the background; ignore a second click (or Return)
		// while it is in flight instead of opening the session twice.
		if connecting {
			return
		}
		profile, password, buildErr := buildProfile()
		if buildErr != nil {
			showErrorBox(dlg, "Connect", buildErr)
			return
		}
		connecting = true
		connectBtn.SetEnabled(false)
		_ = connectBtn.SetText("Connecting…")
		go func() {
			_, connectErr := u.manager.Connect(profile, password, time.Duration(profile.Options.LoginTimeoutMs)*time.Millisecond)
			dlg.Synchronize(func() {
				connecting = false
				connectBtn.SetEnabled(true)
				_ = connectBtn.SetText("&Connect")
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
	connected := dlg.Run() == walk.DlgCmdOK
	if connected {
		// Fill the schema browser right away (cached per session; F8 refreshes).
		u.loadSchema(false)
	}
	// Back in the main window: put the caret in the editor so the user can type SQL right away.
	if tab := u.currentEditorTab(); tab != nil {
		_ = tab.Editor.SetFocus()
	}
	u.refreshState()
}
func (u *applicationUI) showAbout() {
	var report strings.Builder
	report.WriteString("GoQueryOne " + buildinfo.Version + "\n")
	report.WriteString("Commit: " + buildinfo.Commit + "\n")
	report.WriteString("Build date: " + buildinfo.Date + "\n")
	report.WriteString("Architecture: " + buildinfo.Arch() + " (" + runtime.GOARCH + ")\n")
	report.WriteString("License: MIT\n")
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

// updateStatus ends any transient activity/execution and re-renders the
// presenter state (status bar sections, enablement, title).
func (u *applicationUI) updateStatus() {
	u.executing = false
	u.activityText = ""
	u.refreshState()
}

func (u *applicationUI) beginExecuting(activity string) {
	u.executing = true
	u.executingSince = time.Now()
	u.activityText = activity
	u.refreshState()
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
	_ = layout.SetMargins(walk.Margins{HNear: 8, VNear: 8, HFar: 8, VFar: 8})
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
	_ = layout.SetMargins(walk.Margins{HNear: 8, VNear: 8, HFar: 8, VFar: 8})
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
	text := editor.Text()
	bs, be := core.UTF16OffsetToByteOffset(text, start), core.UTF16OffsetToByteOffset(text, end)
	if bs < 0 || be > len(text) || bs >= be {
		return ""
	}
	return text[bs:be]
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

// identQuote is the active connection's identifier quote character
// (SQLGetInfo), or "" when unknown.
func (u *applicationUI) identQuote() string {
	session, err := u.manager.ActiveSession()
	if err != nil {
		return ""
	}
	info, err := u.manager.DBInfo(session.ID)
	if err != nil {
		return ""
	}
	return info.IdentifierQuote
}

// sqlName is a table name ready for generated SQL (quoted when needed).
func (u *applicationUI) sqlName(t odbc.SchemaTable) string {
	return core.QualifiedName(t.Schema, t.Name, u.identQuote())
}

func executablePath() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return exe
}

// bitnessWord is "64-bit" or "32-bit" for the running exe.
func bitnessWord() string {
	if strconv.IntSize == 32 {
		return "32-bit"
	}
	return "64-bit"
}
