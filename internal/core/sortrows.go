package core

import (
	"sort"
	"strconv"
	"strings"
)

// RowSorter reorders a result grid in place (client-side sort on a header
// click). Rows, Nulls and Orig are permuted together, so callers that index
// into them by display row stay consistent. Orig holds each row's original
// position and lets SortOriginal restore the order in which the rows arrived.
type RowSorter struct {
	Rows  [][]string
	Nulls [][]bool // may be nil or shorter than Rows
	Orig  []int    // same length as Rows
}

// NewRowSorter wraps rows and nulls with a fresh identity Orig slice.
func NewRowSorter(rows [][]string, nulls [][]bool) *RowSorter {
	orig := make([]int, len(rows))
	for i := range orig {
		orig[i] = i
	}
	return &RowSorter{Rows: rows, Nulls: nulls, Orig: orig}
}

func (s *RowSorter) isNull(row, col int) bool {
	return row < len(s.Nulls) && col < len(s.Nulls[row]) && s.Nulls[row][col]
}

func (s *RowSorter) cell(row, col int) string {
	if col < len(s.Rows[row]) {
		return s.Rows[row][col]
	}
	return ""
}

func (s *RowSorter) swap(i, j int) {
	s.Rows[i], s.Rows[j] = s.Rows[j], s.Rows[i]
	if i < len(s.Nulls) && j < len(s.Nulls) {
		s.Nulls[i], s.Nulls[j] = s.Nulls[j], s.Nulls[i]
	}
	s.Orig[i], s.Orig[j] = s.Orig[j], s.Orig[i]
}

// SortOriginal restores the original row order.
func (s *RowSorter) SortOriginal() {
	sort.Stable(sorter{s: s, less: func(i, j int) bool { return s.Orig[i] < s.Orig[j] }})
}

// SortByColumn sorts by data column col (0-based). NULLs sort first in
// ascending order (last when descending). When every non-NULL value in the
// column parses as a number the column is compared numerically, otherwise
// case-insensitively as text. The sort is stable.
func (s *RowSorter) SortByColumn(col int, desc bool) {
	if col < 0 {
		s.SortOriginal()
		return
	}
	numeric := true
	nums := make(map[int]float64, len(s.Rows))
	for i := range s.Rows {
		if s.isNull(i, col) {
			continue
		}
		f, err := strconv.ParseFloat(strings.TrimSpace(s.cell(i, col)), 64)
		if err != nil {
			numeric = false
			break
		}
		nums[s.Orig[i]] = f
	}
	cmp := func(i, j int) int {
		ni, nj := s.isNull(i, col), s.isNull(j, col)
		switch {
		case ni && nj:
			return 0
		case ni:
			return -1
		case nj:
			return 1
		}
		if numeric {
			a, b := nums[s.Orig[i]], nums[s.Orig[j]]
			switch {
			case a < b:
				return -1
			case a > b:
				return 1
			}
			return 0
		}
		a, b := strings.ToLower(s.cell(i, col)), strings.ToLower(s.cell(j, col))
		if c := strings.Compare(a, b); c != 0 {
			return c
		}
		return strings.Compare(s.cell(i, col), s.cell(j, col))
	}
	sort.Stable(sorter{s: s, less: func(i, j int) bool {
		if desc {
			return cmp(i, j) > 0
		}
		return cmp(i, j) < 0
	}})
}

type sorter struct {
	s    *RowSorter
	less func(i, j int) bool
}

func (x sorter) Len() int           { return len(x.s.Rows) }
func (x sorter) Less(i, j int) bool { return x.less(i, j) }
func (x sorter) Swap(i, j int)      { x.s.swap(i, j) }
