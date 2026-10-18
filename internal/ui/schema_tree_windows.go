//go:build windows

package ui

import (
	"strings"

	"github.com/lxn/walk"

	"github.com/franklinjr12/GoQueryOne/internal/app"
	"github.com/franklinjr12/GoQueryOne/internal/core"
	"github.com/franklinjr12/GoQueryOne/internal/odbc"
)

// schemaItem adapts app.SchemaNode to walk.TreeItem.
type schemaItem struct {
	node     *app.SchemaNode
	parent   *schemaItem
	children []*schemaItem
}

func (i *schemaItem) Text() string { return i.node.Label }

func (i *schemaItem) Parent() walk.TreeItem {
	if i.parent == nil {
		return nil // a typed nil would not compare equal to nil inside walk
	}
	return i.parent
}

func (i *schemaItem) ChildCount() int                 { return len(i.children) }
func (i *schemaItem) ChildAt(index int) walk.TreeItem { return i.children[index] }
func (i *schemaItem) HasChild() bool                  { return len(i.children) > 0 }

type schemaTreeModel struct {
	walk.TreeModelBase
	roots []*schemaItem
}

func (m *schemaTreeModel) LazyPopulation() bool           { return false }
func (m *schemaTreeModel) RootCount() int                 { return len(m.roots) }
func (m *schemaTreeModel) RootAt(index int) walk.TreeItem { return m.roots[index] }

func wrapSchemaNodes(nodes []*app.SchemaNode, parent *schemaItem) []*schemaItem {
	items := make([]*schemaItem, 0, len(nodes))
	for _, n := range nodes {
		it := &schemaItem{node: n, parent: parent}
		it.children = wrapSchemaNodes(n.Children, it)
		items = append(items, it)
	}
	return items
}

// buildSchemaTree creates the schema browser: groups (Tables, Views, …) with
// tables and their columns. Double-click a table for a SELECT TOP query, a
// column to insert its name at the caret; right-click for more actions.
func (u *applicationUI) buildSchemaTree(parent walk.Container, layout *walk.BoxLayout) error {
	tree, err := walk.NewTreeView(parent)
	if err != nil {
		return err
	}
	u.schemaTree = tree
	_ = layout.SetStretchFactor(tree, 1)

	tree.CurrentItemChanged().Attach(func() {
		node := u.currentSchemaNode()
		if node == nil || node.Table < 0 {
			u.selectedSchemaID = -1
			return
		}
		changed := u.selectedSchemaID != node.Table
		u.selectedSchemaID = node.Table
		if changed && node.Table < len(u.schemaTables) {
			u.showTableDetails(u.schemaTables[node.Table])
		}
	})
	tree.ItemActivated().Attach(func() {
		node := u.currentSchemaNode()
		if node == nil {
			return
		}
		switch node.Kind {
		case app.NodeTable:
			u.generateQuickQuery("top")
		case app.NodeColumn:
			u.insertAtCaret(core.QuoteIdent(node.Column, u.identQuote()))
		}
	})

	menu, err := walk.NewMenu()
	if err != nil {
		return err
	}
	add := func(text string, fn func()) {
		a := walk.NewAction()
		_ = a.SetText(text)
		a.Triggered().Attach(fn)
		_ = menu.Actions().Add(a)
	}
	add("SELECT TOP 100", func() { u.generateQuickQuery("top") })
	add("SELECT *", func() { u.generateQuickQuery("all") })
	add("SELECT COUNT(*)", func() { u.generateQuickQuery("count") })
	_ = menu.Actions().Add(walk.NewSeparatorAction())
	add("Insert name at caret", func() {
		if node := u.currentSchemaNode(); node != nil && node.Kind == app.NodeColumn {
			u.insertAtCaret(core.QuoteIdent(node.Column, u.identQuote()))
		} else if t := u.selectedSchemaTable(); t != nil {
			u.insertAtCaret(u.sqlName(*t))
		}
	})
	add("Copy name", func() {
		if node := u.currentSchemaNode(); node != nil && node.Kind == app.NodeColumn {
			_ = walk.Clipboard().SetText(core.QuoteIdent(node.Column, u.identQuote()))
		} else if t := u.selectedSchemaTable(); t != nil {
			_ = walk.Clipboard().SetText(u.sqlName(*t))
		}
	})
	_ = menu.Actions().Add(walk.NewSeparatorAction())
	add("Refresh schema\tF8", func() { u.loadSchema(true) })
	tree.SetContextMenu(menu)
	return nil
}

func (u *applicationUI) currentSchemaNode() *app.SchemaNode {
	if u.schemaTree == nil {
		return nil
	}
	if it, ok := u.schemaTree.CurrentItem().(*schemaItem); ok && it != nil {
		return it.node
	}
	return nil
}

// setSchemaSnapshot shows a (possibly filtered) schema snapshot in the tree.
// Groups are expanded; tables stay collapsed.
func (u *applicationUI) setSchemaSnapshot(snapshot odbc.SchemaSnapshot) {
	u.schemaSnapshot = snapshot
	u.schemaTables = snapshot.Tables
	u.selectedSchemaID = -1
	_ = u.schemaDetails.SetText("")
	if u.schemaTree == nil {
		return
	}
	model := &schemaTreeModel{roots: wrapSchemaNodes(app.BuildSchemaTree(snapshot, u.cfg.App.HideSystemTables), nil)}
	_ = u.schemaTree.SetModel(model)
	filtering := strings.TrimSpace(u.schemaSearchEntry.Text()) != ""
	for _, g := range model.roots {
		_ = u.schemaTree.SetExpanded(g, true)
		if filtering {
			// Show the matching columns right away while searching.
			for _, t := range g.children {
				_ = u.schemaTree.SetExpanded(t, true)
			}
		}
	}
}

// insertAtCaret replaces the editor selection (or inserts at the caret) with
// text and puts the caret after it.
func (u *applicationUI) insertAtCaret(text string) {
	tab := u.currentEditorTab()
	if tab == nil || text == "" {
		return
	}
	cur := tab.Editor.Text()
	start, end := tab.Editor.TextSelection()
	bs, be := core.UTF16OffsetToByteOffset(cur, start), core.UTF16OffsetToByteOffset(cur, end)
	if bs > be {
		bs, be = be, bs
	}
	next := cur[:bs] + text + cur[be:]
	_ = tab.Editor.SetText(next)
	caret := core.ByteOffsetToUTF16Offset(next, bs+len(text))
	tab.Editor.SetTextSelection(caret, caret)
	_ = tab.Editor.SetFocus()
	u.refreshState()
}
