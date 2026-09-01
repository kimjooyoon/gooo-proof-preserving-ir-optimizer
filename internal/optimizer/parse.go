package optimizer

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

func Digest(raw []byte) string {
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func ParseContract(text string) (Contract, error) {
	contract := Contract{Normalization: map[string]string{}}
	scanner := bufio.NewScanner(strings.NewReader(text))
	seenHeader := false
	inside := false
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := stripMetaComment(scanner.Text())
		if strings.TrimSpace(line) == "" {
			continue
		}
		tokens, err := tokenize(line)
		if err != nil {
			return Contract{}, fmt.Errorf("line %d: %w", lineNo, err)
		}
		if !seenHeader {
			if len(tokens) != 3 || tokens[0] != "contract" || tokens[2] != "{" {
				return Contract{}, fmt.Errorf("line %d: expected contract schema header", lineNo)
			}
			contract.Schema = tokens[1]
			seenHeader = true
			inside = true
			continue
		}
		if len(tokens) == 1 && tokens[0] == "}" {
			if !inside {
				return Contract{}, fmt.Errorf("line %d: unexpected closing brace", lineNo)
			}
			inside = false
			continue
		}
		if !inside {
			return Contract{}, fmt.Errorf("line %d: content after contract body", lineNo)
		}
		if len(tokens) == 0 {
			continue
		}
		switch tokens[0] {
		case "authority":
			if len(tokens) != 2 {
				return Contract{}, fmt.Errorf("line %d: authority requires one value", lineNo)
			}
			contract.Authority = tokens[1]
		case "grammar":
			contract.Grammar = append([]string(nil), tokens[1:]...)
		case "version", "language":
			if len(tokens) != 2 {
				return Contract{}, fmt.Errorf("line %d: %s requires one value", lineNo, tokens[0])
			}
			if tokens[0] == "version" {
				contract.Version = tokens[1]
			} else {
				contract.Language = tokens[1]
			}
		case "precedence":
			contract.Precedence = make([]Decision, 0, len(tokens)-1)
			for _, token := range tokens[1:] {
				contract.Precedence = append(contract.Precedence, Decision(token))
			}
		case "unknown_fields":
			contract.UnknownFields = append([]string(nil), tokens[1:]...)
		case "denominator":
			if len(tokens) != 4 || tokens[2] != "count" {
				return Contract{}, fmt.Errorf("line %d: malformed denominator", lineNo)
			}
			count, err := strconv.Atoi(tokens[3])
			if err != nil {
				return Contract{}, fmt.Errorf("line %d: malformed denominator count", lineNo)
			}
			contract.DenominatorID, contract.DenominatorCount = tokens[1], count
		case "normalization":
			pairs, err := pairsAfter(tokens, 1)
			if err != nil {
				return Contract{}, fmt.Errorf("line %d: %w", lineNo, err)
			}
			for key, value := range pairs {
				contract.Normalization[key] = value
			}
		case "type":
			pairs, err := pairsAfter(tokens, 2)
			if len(tokens) != 4 || err != nil || tokens[2] != "kind" {
				return Contract{}, fmt.Errorf("line %d: malformed type declaration", lineNo)
			}
			contract.Types = append(contract.Types, TypeDecl{Name: tokens[1], Kind: pairs["kind"]})
		case "effect":
			pairs, err := pairsAfter(tokens, 2)
			if len(tokens) != 4 || err != nil || tokens[2] != "kind" {
				return Contract{}, fmt.Errorf("line %d: malformed effect declaration", lineNo)
			}
			contract.Effects = append(contract.Effects, EffectDecl{Name: tokens[1], Kind: pairs["kind"]})
		case "capability":
			pairs, err := pairsAfter(tokens, 2)
			if len(tokens) != 4 || err != nil || tokens[2] != "kind" {
				return Contract{}, fmt.Errorf("line %d: malformed capability declaration", lineNo)
			}
			contract.Capabilities = append(contract.Capabilities, CapabilityDecl{Name: tokens[1], Kind: pairs["kind"]})
		case "forbidden_effect":
			if len(tokens) != 4 || tokens[2] != "when" {
				return Contract{}, fmt.Errorf("line %d: malformed forbidden effect declaration", lineNo)
			}
			contract.ForbiddenEffects = append(contract.ForbiddenEffects, ForbiddenEffect{Effect: tokens[1], When: tokens[3]})
		case "origin_map":
			if len(tokens) != 5 || tokens[3] != "preservation" {
				return Contract{}, fmt.Errorf("line %d: malformed origin map declaration", lineNo)
			}
			contract.OriginMap = OriginMapDecl{Input: tokens[1], Output: tokens[2], Preservation: tokens[4]}
		case "rewrite":
			if len(tokens) < 8 {
				return Contract{}, fmt.Errorf("line %d: malformed rewrite declaration", lineNo)
			}
			pairs, err := pairsAfter(tokens, 2)
			if err != nil {
				return Contract{}, fmt.Errorf("line %d: %w", lineNo, err)
			}
			contract.Rewrites = append(contract.Rewrites, RewriteRule{Name: tokens[1], Rule: pairs["rule"], Requires: pairs["requires"], Preserves: pairs["preserves"]})
		case "obligation":
			if len(tokens) < 3 {
				return Contract{}, fmt.Errorf("line %d: obligation id is missing", lineNo)
			}
			pairs, err := pairsAfter(tokens, 2)
			if err != nil {
				return Contract{}, fmt.Errorf("line %d: %w", lineNo, err)
			}
			contract.ProofObligations = append(contract.ProofObligations, ProofObligation{ID: tokens[1], Stage: pairs["stage"], Step: pairs["step"], Proof: pairs["proof"], Missing: pairs["missing"]})
		case "cost_observation":
			pairs, err := pairsAfter(tokens, 1)
			if err != nil {
				return Contract{}, fmt.Errorf("line %d: %w", lineNo, err)
			}
			contract.Cost = CostPolicy{Vector: pairs["vector"], Pair: pairs["pair"], Policy: pairs["policy"]}
		case "authority_rule":
			pairs, err := pairsAfter(tokens, 1)
			if err != nil {
				return Contract{}, fmt.Errorf("line %d: %w", lineNo, err)
			}
			contract.AuthorityRule, err = parseAuthorityRule(pairs)
			if err != nil {
				return Contract{}, fmt.Errorf("line %d: %w", lineNo, err)
			}
		case "program":
			if len(tokens) < 3 {
				return Contract{}, fmt.Errorf("line %d: program id is missing", lineNo)
			}
			pairs, err := pairsAfter(tokens, 2)
			if err != nil {
				return Contract{}, fmt.Errorf("line %d: %w", lineNo, err)
			}
			result, err := strconv.ParseInt(pairs["result"], 10, 64)
			if err != nil {
				return Contract{}, fmt.Errorf("line %d: malformed program result", lineNo)
			}
			expr, err := parseExpr(pairs["expr"])
			if err != nil {
				return Contract{}, fmt.Errorf("line %d: program %s: %w", lineNo, tokens[1], err)
			}
			assignOrigins(expr, pairs["origin"], "root")
			contract.Programs = append(contract.Programs, ProgramDecl{ID: tokens[1], ExprText: pairs["expr"], Type: pairs["type"], Effect: pairs["effect"], Capability: pairs["capability"], Result: result, Reason: pairs["reason"], Origin: pairs["origin"], Expr: expr})
		case "scenario":
			if len(tokens) < 3 {
				return Contract{}, fmt.Errorf("line %d: scenario id is missing", lineNo)
			}
			pairs, err := pairsAfter(tokens, 2)
			if err != nil {
				return Contract{}, fmt.Errorf("line %d: %w", lineNo, err)
			}
			replay, err := strconv.ParseBool(pairs["replay"])
			if err != nil {
				return Contract{}, fmt.Errorf("line %d: malformed replay value", lineNo)
			}
			contract.Scenarios = append(contract.Scenarios, Scenario{ID: tokens[1], Expected: Decision(pairs["expected"]), Source: pairs["source"], Operator: pairs["operator"], Program: pairs["program"], Variant: pairs["variant"], Replay: replay})
		default:
			return Contract{}, fmt.Errorf("line %d: unknown contract record %q", lineNo, tokens[0])
		}
	}
	if err := scanner.Err(); err != nil {
		return Contract{}, err
	}
	if !seenHeader || inside {
		return Contract{}, fmt.Errorf("incomplete contract declaration")
	}
	if err := ValidateContract(contract); err != nil {
		return Contract{}, err
	}
	return contract, nil
}

func ValidateContract(contract Contract) error {
	if contract.Schema != "gooo-proof-preserving-ir-optimizer/v1" || contract.Authority != "metacode" || contract.Version != "1" || contract.Language != "go" {
		return fmt.Errorf("contract identity or metacode authority is invalid")
	}
	if strings.Join(contract.Grammar, ",") != "expr-v1,records-v1,quoted-values-v1" {
		return fmt.Errorf("contract grammar is not the declared grammar")
	}
	if strings.Join(contract.Precedence, ",") != "REFUTED,UNKNOWN,CLOSED" {
		return fmt.Errorf("contract precedence must be REFUTED > UNKNOWN > CLOSED")
	}
	if strings.Join(contract.UnknownFields, ",") != "stage,step,reason,unknown_class,next_operation,blocked_by" {
		return fmt.Errorf("UNKNOWN tuple is incomplete")
	}
	if contract.DenominatorID == "" || contract.DenominatorCount != 8 || len(contract.Scenarios) != 8 {
		return fmt.Errorf("fixed denominator must contain exactly eight scenarios")
	}
	if len(contract.Types) != 3 || len(contract.Effects) != 4 || len(contract.Capabilities) != 3 || len(contract.ForbiddenEffects) != 3 || len(contract.Rewrites) != 3 || len(contract.ProofObligations) != 7 || len(contract.Programs) != 4 {
		return fmt.Errorf("typed IR, effect, capability, rewrite, proof, or program declarations are incomplete")
	}
	if contract.OriginMap.Input != "source-origin" || contract.OriginMap.Output != "derived-origin" || contract.OriginMap.Preservation != "all-source-anchors" {
		return fmt.Errorf("origin map declaration is incomplete")
	}
	if contract.Cost.Vector != "binary_bytes,generated_bytes,wall_ms,peak_rss_kib" || contract.Cost.Pair != "same-scenario-source-contract-toolchain-runner" || contract.Cost.Policy != "exact-before-after-per-closed-case" {
		return fmt.Errorf("cost observation policy is incomplete")
	}
	if contract.AuthorityRule.RepositoryWrites != 0 || contract.AuthorityRule.OutputScope != "CALLER_OWNED_TEMP_OUTPUT_ONLY" || contract.AuthorityRule.AutomaticCommit != 0 || contract.AuthorityRule.AutomaticPush != 0 || contract.AuthorityRule.AutomaticMerge != 0 || contract.AuthorityRule.AutomaticRelease != 0 {
		return fmt.Errorf("authority boundary is not zero-write")
	}
	seenPrograms := map[string]bool{}
	for _, program := range contract.Programs {
		if program.ID == "" || seenPrograms[program.ID] || program.Expr == nil || program.Type == "" || program.Effect == "" || program.Capability == "" || program.Origin == "" {
			return fmt.Errorf("program declaration is incomplete or duplicated")
		}
		seenPrograms[program.ID] = true
		if program.Expr.Type != program.Type || program.Expr.Effect != program.Effect || program.Expr.Capability != program.Capability {
			return fmt.Errorf("program %q metadata disagrees with typed IR", program.ID)
		}
	}
	seenScenarios := map[string]bool{}
	for _, scenario := range contract.Scenarios {
		if scenario.ID == "" || seenScenarios[scenario.ID] || scenario.Source == "" || scenario.Operator == "" || scenario.Program == "" || scenario.Variant == "" {
			return fmt.Errorf("scenario declaration is incomplete or duplicated")
		}
		if scenario.Expected != Closed && scenario.Expected != Unknown && scenario.Expected != Refuted {
			return fmt.Errorf("scenario %q has an invalid expected state", scenario.ID)
		}
		seenScenarios[scenario.ID] = true
	}
	return nil
}

func parseAuthorityRule(pairs map[string]string) (AuthorityRule, error) {
	read := func(key string) (int, error) {
		value, err := strconv.Atoi(pairs[key])
		if err != nil {
			return 0, fmt.Errorf("malformed authority_rule %s", key)
		}
		return value, nil
	}
	commit, err := read("automatic_commit")
	if err != nil {
		return AuthorityRule{}, err
	}
	push, err := read("automatic_push")
	if err != nil {
		return AuthorityRule{}, err
	}
	merge, err := read("automatic_merge")
	if err != nil {
		return AuthorityRule{}, err
	}
	release, err := read("automatic_release")
	if err != nil {
		return AuthorityRule{}, err
	}
	writes, err := read("repository_writes")
	if err != nil {
		return AuthorityRule{}, err
	}
	return AuthorityRule{RepositoryWrites: writes, OutputScope: pairs["output_scope"], AutomaticCommit: commit, AutomaticPush: push, AutomaticMerge: merge, AutomaticRelease: release}, nil
}

func ContractCanonical(contract Contract) string {
	clone := contract
	clone.SourceDigest = ""
	clone.SourceCanonicalKey = ""
	data, _ := json.Marshal(struct {
		Schema           string
		Authority        string
		Grammar          []string
		Version          string
		Language         string
		Precedence       []Decision
		UnknownFields    []string
		DenominatorID    string
		DenominatorCount int
		Normalization    map[string]string
		Types            []TypeDecl
		Effects          []EffectDecl
		Capabilities     []CapabilityDecl
		ForbiddenEffects []ForbiddenEffect
		OriginMap        OriginMapDecl
		Rewrites         []RewriteRule
		ProofObligations []ProofObligation
		Cost             CostPolicy
		AuthorityRule    AuthorityRule
		Programs         []ProgramDecl
		Scenarios        []Scenario
	}{clone.Schema, clone.Authority, clone.Grammar, clone.Version, clone.Language, clone.Precedence, clone.UnknownFields, clone.DenominatorID, clone.DenominatorCount, clone.Normalization, clone.Types, clone.Effects, clone.Capabilities, clone.ForbiddenEffects, clone.OriginMap, clone.Rewrites, clone.ProofObligations, clone.Cost, clone.AuthorityRule, clone.Programs, clone.Scenarios})
	return string(data)
}

func parseExpr(text string) (*Expr, error) {
	p := expressionParser{text: text}
	expr, err := p.parse()
	if err != nil {
		return nil, err
	}
	if p.pos != len(p.text) {
		return nil, fmt.Errorf("unexpected expression suffix %q", p.text[p.pos:])
	}
	return expr, nil
}

type expressionParser struct {
	text string
	pos  int
}

func (p *expressionParser) parse() (*Expr, error) {
	name := p.identifier()
	if name == "" {
		return nil, fmt.Errorf("expression function is missing at byte %d", p.pos)
	}
	if p.pos >= len(p.text) || p.text[p.pos] != '(' {
		return nil, fmt.Errorf("expression %q is missing opening parenthesis", name)
	}
	p.pos++
	switch name {
	case "const":
		literal := p.until(')')
		if literal == "" {
			return nil, fmt.Errorf("const literal is empty")
		}
		if literal == "true" || literal == "false" {
			p.pos++
			return &Expr{Kind: "const-bool", Type: "Bool", Effect: "PURE", Capability: "NONE", BoolValue: literal == "true", HasBool: true}, nil
		}
		value, err := strconv.ParseInt(literal, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid const literal %q", literal)
		}
		p.pos++
		return &Expr{Kind: "const-int", Type: "Int", Effect: "PURE", Capability: "NONE", IntValue: value}, nil
	case "var":
		name := p.until(')')
		if name == "" {
			return nil, fmt.Errorf("var name is empty")
		}
		p.pos++
		return &Expr{Kind: "var", Name: name, Type: "Int", Effect: "PURE", Capability: "NONE"}, nil
	case "effect":
		effectName := p.identifier()
		if effectName == "" || p.pos >= len(p.text) || p.text[p.pos] != ',' {
			return nil, fmt.Errorf("effect requires name and operand")
		}
		p.pos++
		operand, err := p.parse()
		if err != nil {
			return nil, err
		}
		if p.pos >= len(p.text) || p.text[p.pos] != ')' {
			return nil, fmt.Errorf("effect is missing closing parenthesis")
		}
		p.pos++
		return &Expr{Kind: "effect", Name: effectName, Args: []*Expr{operand}, Type: operand.Type, Effect: "IO", Capability: "OUTPUT"}, nil
	case "add", "mul", "sub", "eq", "if":
		args := make([]*Expr, 0, 2)
		for {
			arg, err := p.parse()
			if err != nil {
				return nil, err
			}
			args = append(args, arg)
			if p.pos >= len(p.text) {
				return nil, fmt.Errorf("%s is missing closing parenthesis", name)
			}
			if p.text[p.pos] == ')' {
				p.pos++
				break
			}
			if p.text[p.pos] != ',' {
				return nil, fmt.Errorf("%s expects comma-separated operands", name)
			}
			p.pos++
		}
		expr := &Expr{Kind: name, Args: args}
		if name == "if" {
			if len(args) != 3 || args[0].Type != "Bool" || args[1].Type != args[2].Type {
				return nil, fmt.Errorf("if requires Bool condition and equal branch types")
			}
			expr.Type = args[1].Type
		} else {
			if len(args) != 2 || args[0].Type != "Int" || args[1].Type != "Int" {
				return nil, fmt.Errorf("%s requires two Int operands", name)
			}
			expr.Type = "Int"
			if name == "eq" {
				expr.Type = "Bool"
			}
		}
		expr.Effect, expr.Capability = "PURE", "NONE"
		for _, arg := range args {
			expr.Effect = joinEffect(expr.Effect, arg.Effect)
			expr.Capability = joinCapability(expr.Capability, arg.Capability)
		}
		return expr, nil
	default:
		return nil, fmt.Errorf("unknown expression function %q", name)
	}
}

func (p *expressionParser) identifier() string {
	start := p.pos
	for p.pos < len(p.text) {
		ch := p.text[p.pos]
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_' || ch == '-' || ch == ':' {
			p.pos++
			continue
		}
		break
	}
	return p.text[start:p.pos]
}

func (p *expressionParser) until(delimiter byte) string {
	start := p.pos
	for p.pos < len(p.text) && p.text[p.pos] != delimiter {
		p.pos++
	}
	return p.text[start:p.pos]
}

func assignOrigins(expr *Expr, root, path string) {
	if expr == nil {
		return
	}
	expr.Origin = root + "." + path
	for index, arg := range expr.Args {
		assignOrigins(arg, root, fmt.Sprintf("%s.%d", path, index))
	}
	assignOrigins(expr.ValueExpr, root, path+".value")
	assignOrigins(expr.BodyExpr, root, path+".body")
}

func stripMetaComment(line string) string {
	inString := false
	escaped := false
	for index := 0; index+1 < len(line); index++ {
		if line[index] == '\\' && inString {
			escaped = !escaped
			continue
		}
		if line[index] == '"' && !escaped {
			inString = !inString
		}
		escaped = false
		if !inString && line[index] == '/' && line[index+1] == '/' {
			return line[:index]
		}
	}
	return line
}

func tokenize(line string) ([]string, error) {
	var tokens []string
	for index := 0; index < len(line); {
		for index < len(line) && (line[index] == ' ' || line[index] == '\t' || line[index] == '\r') {
			index++
		}
		if index == len(line) {
			break
		}
		if line[index] == '"' {
			start := index
			index++
			escaped := false
			closed := false
			for index < len(line) {
				if escaped {
					escaped = false
					index++
					continue
				}
				if line[index] == '\\' {
					escaped = true
					index++
					continue
				}
				if line[index] == '"' {
					index++
					closed = true
					break
				}
				index++
			}
			if !closed {
				return nil, fmt.Errorf("unterminated quoted value")
			}
			value, err := strconv.Unquote(line[start:index])
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, value)
			continue
		}
		start := index
		for index < len(line) && line[index] != ' ' && line[index] != '\t' && line[index] != '\r' {
			index++
		}
		tokens = append(tokens, line[start:index])
	}
	return tokens, nil
}

func pairsAfter(tokens []string, start int) (map[string]string, error) {
	if start > len(tokens) || len(tokens[start:])%2 != 0 {
		return nil, fmt.Errorf("expected key/value pairs")
	}
	pairs := make(map[string]string, len(tokens[start:])/2)
	for index := start; index < len(tokens); index += 2 {
		if pairs[tokens[index]] != "" || tokens[index] == "" {
			return nil, fmt.Errorf("duplicate or malformed key %q", tokens[index])
		}
		pairs[tokens[index]] = tokens[index+1]
	}
	return pairs, nil
}
