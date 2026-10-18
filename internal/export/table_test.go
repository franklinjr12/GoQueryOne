package export

import (
	"reflect"
	"testing"
)

func TestIsNull(t *testing.T) {
	if !tNulls.IsNull(2, 1) {
		t.Error("expected row 2 col 1 to be null")
	}
	if tNulls.IsNull(0, 1) {
		t.Error("expected row 0 col 1 to not be null")
	}
	if tNulls.IsNull(99, 0) {
		t.Error("out-of-range row should not be null")
	}
	if tNulls.IsNull(0, 99) {
		t.Error("out-of-range col should not be null")
	}
	if tNilNulls.IsNull(0, 0) {
		t.Error("nil Nulls should never report null")
	}
}

func TestSelectRows(t *testing.T) {
	got := SelectRows(tNulls, []int{2, 0})
	want := Table{
		Columns: tNulls.Columns,
		Rows:    [][]string{tNulls.Rows[2], tNulls.Rows[0]},
		Nulls:   [][]bool{tNulls.Nulls[2], tNulls.Nulls[0]},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("SelectRows mismatch:\ngot  %+v\nwant %+v", got, want)
	}
}

func TestSelectRowsOutOfRangePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("expected panic for out-of-range row index")
		}
	}()
	SelectRows(tNulls, []int{100})
}

func TestSelectRowsNilNulls(t *testing.T) {
	got := SelectRows(tNilNulls, []int{1})
	if got.Rows[0][0] != "y" {
		t.Errorf("got %+v", got)
	}
	if got.IsNull(0, 0) {
		t.Error("should not be null")
	}
}

func TestUniqueColumnNames(t *testing.T) {
	got := UniqueColumnNames([]string{"name", "", "name", ""})
	want := []string{"name", "column_2", "name_2", "column_4"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v want %v", got, want)
	}
}

func TestUniqueColumnNamesEmpty(t *testing.T) {
	got := UniqueColumnNames(nil)
	if len(got) != 0 {
		t.Errorf("got %v", got)
	}
}

func TestUniqueColumnNamesCollisionWithGenerated(t *testing.T) {
	// A real column named "column_2" should not collide silently with the
	// auto-generated name for the empty column at position 2 (index 1).
	got := UniqueColumnNames([]string{"column_2", ""})
	want := []string{"column_2", "column_2_2"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v want %v", got, want)
	}
}
