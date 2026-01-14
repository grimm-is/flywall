package firewall

import (
	"fmt"
	"strings"
)

// ScriptBuilder builds nftables scripts for atomic application.
type ScriptBuilder struct {
	tableName  string
	family     string
	timezone   string              // Timezone for time-based rules
	lines      []string            // Raw lines (comments, flush commands) - DEPRECATED for structured objects? No, used for sets.
	tables     []string            // Table definitions
	chains     []string            // Chain definitions
	flowtables []string            // Flowtable definitions
	rules      map[string][]string // Rules keyed by chain name (to keep them grouped)
	sets       []string            // Set definitions
	maps       []string            // Map definitions
	counters   []string            // Counter definitions
	chainOrder []string            // Order of chains to output (preserving addition order)
}

func NewScriptBuilder(tableName, family, timezone string) *ScriptBuilder {
	return &ScriptBuilder{
		tableName: tableName,
		family:    family,
		timezone:  timezone,
		rules:     make(map[string][]string),
	}
}

func (sb *ScriptBuilder) AddLine(line string) {
	sb.lines = append(sb.lines, line)
}

func (sb *ScriptBuilder) AddTable() {
	sb.tables = append(sb.tables, fmt.Sprintf("add table %s %s", sb.family, sb.tableName))
}

func (sb *ScriptBuilder) AddTableWithComment(comment string) {
	sb.tables = append(sb.tables, fmt.Sprintf("add table %s %s { comment %q; }", sb.family, sb.tableName, comment))
}

func (sb *ScriptBuilder) AddChain(name, typeName, hook string, priority int, policy string, comment ...string) {
	var cmd string
	if typeName != "" {
		cmd = fmt.Sprintf("add chain %s %s %s { type %s hook %s priority %d; policy %s;",
			sb.family, sb.tableName, name, typeName, hook, priority, policy)
	} else {
		cmd = fmt.Sprintf("add chain %s %s %s {", sb.family, sb.tableName, name)
	}

	if len(comment) > 0 {
		cmd += fmt.Sprintf(" comment %q;", comment[0])
	}
	cmd += " }"
	sb.chains = append(sb.chains, cmd)
	sb.chainOrder = append(sb.chainOrder, name)
}

func (sb *ScriptBuilder) AddRule(chain, rule string, comment ...string) {
	// If rule already has a comment, don't add another one
	if (len(comment) > 0 && comment[0] != "") && !strings.Contains(rule, "comment \"") {
		rule += fmt.Sprintf(" comment %q", comment[0])
	}
	sb.rules[chain] = append(sb.rules[chain], fmt.Sprintf("add rule %s %s %s %s", sb.family, sb.tableName, chain, rule))
}

func (sb *ScriptBuilder) AddSet(name, setType, comment string, size int, flags ...string) {
	typeKeyword := "type"
	if strings.Contains(setType, " ") || strings.Contains(setType, ".") {
		typeKeyword = "typeof"
	}

	def := fmt.Sprintf("add set %s %s %s { %s %s;", sb.family, sb.tableName, quote(name), typeKeyword, setType)
	if len(flags) > 0 {
		def += fmt.Sprintf(" flags %s;", strings.Join(flags, ","))
	}
	if size > 0 {
		def += fmt.Sprintf(" size %d;", size)
	}
	if comment != "" {
		def += fmt.Sprintf(" comment %q;", comment)
	}
	def += " }"
	sb.sets = append(sb.sets, def)
}

func (sb *ScriptBuilder) AddSetElements(setName string, elements []string) {
	// Add elements 100 at a time to avoid huge command lines
	batchSize := 100
	for i := 0; i < len(elements); i += batchSize {
		end := i + batchSize
		if end > len(elements) {
			end = len(elements)
		}

		chunk := elements[i:end]
		// Quote elements if necessary? Usually passed correctly formatted.
		// If elements contain spaces (comments?), nft might parse weirdly.
		// Assuming caller handles basic formatting, but we might verify.
		sb.lines = append(sb.lines, fmt.Sprintf("add element %s %s %s { %s }",
			sb.family, sb.tableName, quote(setName), strings.Join(chunk, ", ")))
	}
}

func (sb *ScriptBuilder) AddMap(name, keyType, valueType, comment string, flags []string, elements []string) {
	def := fmt.Sprintf("add map %s %s %s { type %s : %s;", sb.family, sb.tableName, quote(name), keyType, valueType)
	if len(flags) > 0 {
		def += fmt.Sprintf(" flags %s;", strings.Join(flags, ","))
	}
	if comment != "" {
		def += fmt.Sprintf(" comment %q;", comment)
	}
	if len(elements) > 0 {
		def += fmt.Sprintf(" elements = { %s };", strings.Join(elements, ", "))
	}
	def += " }"
	sb.maps = append(sb.maps, def)
}

func (sb *ScriptBuilder) AddCounter(name, comment string) {
	// Note: nftables counter objects do not support the comment clause.
	// Comments are silently ignored here but kept in the signature for doc purposes.
	_ = comment
	def := fmt.Sprintf("add counter %s %s %s", sb.family, sb.tableName, quote(name))
	sb.counters = append(sb.counters, def)
}

func (sb *ScriptBuilder) AddFlowtable(name string, devices []string, comment ...string) {
	// flowtable ft { hook ingress priority 0; devices = { ... }; }
	// Need to force quote devices
	var qDevices []string
	for _, d := range devices {
		qDevices = append(qDevices, fmt.Sprintf("%q", d))
	}
	def := fmt.Sprintf("add flowtable %s %s %s { hook ingress priority 0; devices = { %s };",
		sb.family, sb.tableName, name, strings.Join(qDevices, ", "))
	if len(comment) > 0 {
		def += fmt.Sprintf(" comment %q;", comment[0])
	}
	def += " }"
	sb.flowtables = append(sb.flowtables, def)
}

func (sb *ScriptBuilder) Build() string {
	var lines []string

	// Comments/Header provided by caller mostly, but tables first
	lines = append(lines, sb.tables...)

	// Sets
	lines = append(lines, sb.sets...)

	// Counters
	lines = append(lines, sb.counters...)

	// Flowtables
	lines = append(lines, sb.flowtables...)

	// Chains (must be before maps that reference them via jump)
	lines = append(lines, sb.chains...)

	// Maps (after chains, since maps may contain jump verdicts)
	lines = append(lines, sb.maps...)

	// Rules (in chain order)
	// We defined specific chain addition order logic: we store chain names in chainOrder.
	// But what if chains added out of order? The caller calls AddChain.
	// We iterate sb.chainOrder to output rules for each chain.
	for _, chain := range sb.chainOrder {
		if rules, ok := sb.rules[chain]; ok {
			lines = append(lines, rules...)
		}
	}

	// Lines (Flush commands, element additions)
	// Usually placed after sets defined.
	// Wait, sets defs are separate from lines?
	// AddLine is generic. AddSetElements appends to AddLine.
	// We should probably interleave correctly.
	// Current impl: AddLine logic is simplistic.
	// If AddSetElements called, it appends to sb.lines.
	// Let's output sb.lines after sets/maps but before chains?
	// Or after chains? Element addition works any time after set exists.
	// Outputting after defines sets is safest.
	lines = append(lines, sb.lines...)

	return strings.Join(lines, "\n") + "\n"
}

// String returns the script for debugging.
func (b *ScriptBuilder) String() string {
	return b.Build()
}
