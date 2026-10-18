package core

import (
	"reflect"
	"testing"
)

func col0(rows [][]string) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r[0]
	}
	return out
}

func TestRowSorterNumericAndNulls(t *testing.T) {
	rows := [][]string{{"10"}, {"9"}, {""}, {"100"}, {"-1.5"}}
	nulls := [][]bool{{false}, {false}, {true}, {false}, {false}}
	s := NewRowSorter(rows, nulls)
	s.SortByColumn(0, false)
	if got := col0(rows); !reflect.DeepEqual(got, []string{"", "-1.5", "9", "10", "100"}) {
		t.Fatalf("asc = %v", got)
	}
	if !nulls[0][0] || nulls[1][0] {
		t.Fatalf("nulls not permuted with rows: %v", nulls)
	}
	s.SortByColumn(0, true)
	if got := col0(rows); !reflect.DeepEqual(got, []string{"100", "10", "9", "-1.5", ""}) {
		t.Fatalf("desc = %v", got)
	}
	s.SortOriginal()
	if got := col0(rows); !reflect.DeepEqual(got, []string{"10", "9", "", "100", "-1.5"}) {
		t.Fatalf("original = %v", got)
	}
	if !reflect.DeepEqual(s.Orig, []int{0, 1, 2, 3, 4}) || !nulls[2][0] {
		t.Fatalf("orig=%v nulls=%v", s.Orig, nulls)
	}
}

func TestRowSorterTextStableCaseInsensitive(t *testing.T) {
	rows := [][]string{{"b", "1"}, {"A", "2"}, {"a", "3"}, {"10", "4"}, {"B", "5"}}
	s := NewRowSorter(rows, nil)
	s.SortByColumn(0, false) // mixed text: "10" is compared as text
	want := [][]string{{"10", "4"}, {"A", "2"}, {"a", "3"}, {"B", "5"}, {"b", "1"}}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("got %v", rows)
	}
	s.SortByColumn(1, true)
	if got := col0(rows); !reflect.DeepEqual(got, []string{"B", "10", "a", "A", "b"}) {
		t.Fatalf("by col 1 desc = %v", got)
	}
	s.SortByColumn(-1, false)
	if got := col0(rows); !reflect.DeepEqual(got, []string{"b", "A", "a", "10", "B"}) {
		t.Fatalf("original = %v", got)
	}
}

func TestRowSorterShortRowsAndEmpty(t *testing.T) {
	NewRowSorter(nil, nil).SortByColumn(0, false)
	rows := [][]string{{"x", "2"}, {"y"}, {"z", "1"}}
	s := NewRowSorter(rows, [][]bool{{false, false}})
	s.SortByColumn(1, false) // missing cell reads as "" (not numeric) -> text sort
	if got := col0(rows); !reflect.DeepEqual(got, []string{"y", "z", "x"}) {
		t.Fatalf("got %v", got)
	}
}
