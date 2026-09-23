package optimizer

import "testing"

func TestParseExprRejectsUnclosedLeafExpressions(t *testing.T) {
	for _, input := range []string{"const(1", "var(x"} {
		if _, err := parseExpr(input); err == nil {
			t.Fatalf("parseExpr(%q) accepted an unclosed expression", input)
		}
	}
}
