package optimizer

import "testing"

func TestParseExprRejectsMissingClosingParenthesis(t *testing.T) {
	for _, input := range []string{"const(1", "const(true", "var(x"} {
		t.Run(input, func(t *testing.T) {
			if _, err := parseExpr(input); err == nil {
				t.Fatalf("parseExpr accepted malformed input %q", input)
			}
		})
	}
}
