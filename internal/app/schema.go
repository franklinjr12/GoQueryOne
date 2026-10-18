package app

import (
	"fmt"
	"sort"
	"strings"

	"github.com/franklinjr12/GoQueryOne/internal/odbc"
)

// SchemaNodeKind tells what a schema tree node stands for.
type SchemaNodeKind int

const (
	NodeGroup  SchemaNodeKind = iota // "Tables (12)", "Views (3)", …
	NodeTable                        // a table or view
	NodeColumn                       // a column of its parent table
)

// SchemaNode is one node of the schema browser tree.
type SchemaNode struct {
	Kind     SchemaNodeKind
	Label    string
	Table    int    // index into the snapshot's Tables (table and column nodes), -1 for groups
	Column   string // column name (column nodes)
	Parent   *SchemaNode
	Children []*SchemaNode
}

var groupOrder = []string{"Tables", "Views", "Synonyms", "System tables", "Other"}

// groupFor maps an ODBC TABLE_TYPE to the tree group it is listed under.
func groupFor(tableType string) string {
	switch t := strings.ToUpper(strings.TrimSpace(tableType)); {
	case t == "TABLE" || t == "BASE TABLE" || t == "":
		return "Tables"
	case strings.Contains(t, "VIEW"):
		return "Views"
	case t == "SYNONYM" || t == "ALIAS":
		return "Synonyms"
	case strings.HasPrefix(t, "SYSTEM") || t == "GLOBAL TEMPORARY" || t == "LOCAL TEMPORARY":
		return "System tables"
	default:
		return "Other"
	}
}

// TableLabel is how a table is shown: schema.name, or just name.
func TableLabel(t odbc.SchemaTable) string {
	if strings.TrimSpace(t.Schema) == "" {
		return t.Name
	}
	return t.Schema + "." + t.Name
}

// ColumnLabel shows a column as "name  TYPE(size), not null".
func ColumnLabel(c odbc.SchemaColumn) string {
	typ := strings.TrimSpace(c.Type)
	if c.Size > 0 && typ != "" && sizedType(typ) {
		typ = fmt.Sprintf("%s(%d)", typ, c.Size)
	}
	label := c.Name
	if typ != "" {
		label += "  " + typ
	}
	if !c.Nullable {
		label += ", not null"
	}
	return label
}

// sizedType reports whether the size is worth showing for a type
// (character and binary types; not TEXT/BLOB-like or numeric ones).
func sizedType(t string) bool {
	t = strings.ToUpper(t)
	return strings.Contains(t, "CHAR") || strings.Contains(t, "BINARY")
}

// BuildSchemaTree groups the snapshot's tables by type (Tables, Views,
// Synonyms, System tables, Other) with their columns as children. Empty
// groups are left out; tables are sorted case-insensitively.
func BuildSchemaTree(snap odbc.SchemaSnapshot) []*SchemaNode {
	byGroup := map[string][]int{}
	for i, t := range snap.Tables {
		g := groupFor(t.Type)
		byGroup[g] = append(byGroup[g], i)
	}
	var roots []*SchemaNode
	for _, g := range groupOrder {
		idx := byGroup[g]
		if len(idx) == 0 {
			continue
		}
		sort.SliceStable(idx, func(a, b int) bool {
			return strings.ToLower(TableLabel(snap.Tables[idx[a]])) < strings.ToLower(TableLabel(snap.Tables[idx[b]]))
		})
		group := &SchemaNode{Kind: NodeGroup, Label: fmt.Sprintf("%s (%d)", g, len(idx)), Table: -1}
		for _, i := range idx {
			t := snap.Tables[i]
			tn := &SchemaNode{Kind: NodeTable, Label: TableLabel(t), Table: i, Parent: group}
			for _, c := range snap.ColumnsOf(t) {
				tn.Children = append(tn.Children, &SchemaNode{Kind: NodeColumn, Label: ColumnLabel(c), Table: i, Column: c.Name, Parent: tn})
			}
			group.Children = append(group.Children, tn)
		}
		roots = append(roots, group)
	}
	return roots
}
