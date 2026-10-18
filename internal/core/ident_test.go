package core

import "testing"

func TestQuoteIdent(t *testing.T) {
	cases := []struct{ name, quote, want string }{
		{"orders", `"`, "orders"},
		{"Order_2024$", `"`, "Order_2024$"},
		{"order details", `"`, `"order details"`},
		{`we"ird`, `"`, `"we""ird"`},
		{"2fast", "`", "`2fast`"},
		{"Preço", `"`, `"Preço"`},
		{"my table", "[", "[my table]"},
		{"a]b", "[", "[a]]b]"},
		{"my table", "", "my table"},
		{"my table", " ", "my table"},
		{"", `"`, ""},
	}
	for _, tc := range cases {
		if got := QuoteIdent(tc.name, tc.quote); got != tc.want {
			t.Errorf("QuoteIdent(%q, %q) = %q, want %q", tc.name, tc.quote, got, tc.want)
		}
	}
	if got := QualifiedName("dbo", "order details", `"`); got != `dbo."order details"` {
		t.Errorf("QualifiedName = %q", got)
	}
	if got := QualifiedName("", "t", `"`); got != "t" {
		t.Errorf("QualifiedName no schema = %q", got)
	}
}
