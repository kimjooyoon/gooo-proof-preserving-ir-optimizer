package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/kimjooyoon/gooo-proof-preserving-ir-optimizer/internal/optimizer"
)

func main() {
	if len(os.Args) < 2 {
		fatal("command is required: conformance")
	}
	switch os.Args[1] {
	case "conformance":
		runConformance(os.Args[2:])
	default:
		fatal(fmt.Sprintf("unknown command %q", os.Args[1]))
	}
}

func runConformance(args []string) {
	set := flag.NewFlagSet("conformance", flag.ExitOnError)
	meta := set.String("meta", "", "authoritative .gooo contract")
	root := set.String("root", "", "immutable input repository root")
	out := set.String("out", "", "caller-owned temporary output directory")
	_ = set.Parse(args)
	if *meta == "" || *root == "" || *out == "" {
		fatal("conformance requires --meta, --root, and --out")
	}
	report, err := optimizer.RunConformance(*meta, *root, *out)
	if err != nil {
		fatal(err.Error())
	}
	data, err := json.Marshal(report.Summary)
	if err != nil {
		fatal(err.Error())
	}
	fmt.Println(string(data))
}

func fatal(message string) {
	fmt.Fprintln(os.Stderr, message)
	os.Exit(1)
}
