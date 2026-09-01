//go:build integration

package integration_test

import (
	"os"
	"testing"

	"github.com/kimjooyoon/gooo-proof-preserving-ir-optimizer/internal/optimizer"
)

func TestAuthoritativeMetaContractLoadsForIntegration(t *testing.T) {
	raw, err := os.ReadFile("../.gooo/proof-preserving-ir-optimizer.gooo")
	if err != nil {
		t.Fatal(err)
	}
	contract, err := optimizer.ParseContract(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	if contract.DenominatorCount != 8 || len(contract.Scenarios) != 8 {
		t.Fatalf("unexpected fixed denominator: count=%d scenarios=%d", contract.DenominatorCount, len(contract.Scenarios))
	}
}
