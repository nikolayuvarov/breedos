package organism

import (
	"strings"
)

// renderDiagram emits the §12 verbatim anatomy diagram (lines 1152-1190)
// parameterised by which organs the spec actually materialises.
// Materialised organs render with a "[●]" marker; absent organs render
// with "[ ]" so the practitioner can see at a glance which parts of the
// anatomy are wired.
func renderDiagram(spec PromptOrganismSpec) string {
	mark := func(o OrganName) string {
		if organMaterialised(spec, o) {
			return "[●]"
		}
		return "[ ]"
	}

	var sb strings.Builder
	sb.WriteString("# Prompt Organism Anatomy — " + spec.PromptOrganism.Name + "\n")
	sb.WriteString("# species: " + spec.PromptOrganism.Species + " · size: " + string(spec.PromptOrganism.OrganismType) + "\n")
	sb.WriteString("#\n")
	sb.WriteString("# legend: [●] organ materialised in spec; [ ] organ declared but empty.\n")
	sb.WriteString("#\n")
	valuesMark := "[ ]"
	if len(spec.Values.Primary) > 0 {
		valuesMark = "[●]"
	}
	sb.WriteString("            VALUES " + valuesMark + "\n")
	sb.WriteString("              │\n")
	sb.WriteString("              ▼\n")
	sb.WriteString("        CONSTITUTION " + mark(OrganConstitutional) + "\n")
	sb.WriteString("              │\n")
	sb.WriteString("    ┌─────────┼─────────┐\n")
	sb.WriteString("    ▼         ▼         ▼\n")
	sb.WriteString(" GENOME " + mark(OrganGenome) + "   GOVERNANCE " + mark(OrganGovernance) + "   SECURITY " + mark(OrganImmune) + "\n")
	sb.WriteString("    │         │         │\n")
	sb.WriteString("    └────► RUNTIME " + mark(OrganRuntime) + " ◄────┘\n")
	sb.WriteString("              │\n")
	sb.WriteString("              ▼\n")
	sb.WriteString("   CONTEXT " + mark(OrganContext) + "   MEMORY " + mark(OrganMemory) + "   TOOLS " + mark(OrganTool) + "\n")
	sb.WriteString("              │\n")
	sb.WriteString("              ▼\n")
	sb.WriteString("       METABOLISM " + mark(OrganMetabolic) + "\n")
	sb.WriteString("              │\n")
	sb.WriteString("              ▼\n")
	sb.WriteString("      BELIEF STATE " + mark(OrganEpistemic) + "\n")
	sb.WriteString("              │\n")
	sb.WriteString("              ▼\n")
	sb.WriteString("  DECISION " + mark(OrganDecision) + " + PLANNING " + mark(OrganPlanning) + "\n")
	sb.WriteString("              │\n")
	sb.WriteString("              ▼\n")
	sb.WriteString("    ACTION / OUTPUT DRAFT\n")
	sb.WriteString("              │\n")
	sb.WriteString("              ▼\n")
	sb.WriteString("     VALIDATION GATES (homeostatic loop)\n")
	sb.WriteString("              │\n")
	sb.WriteString("              ▼\n")
	sb.WriteString("        FINAL OUTPUT\n")
	sb.WriteString("              │\n")
	sb.WriteString("              ▼\n")
	sb.WriteString("    OBSERVABILITY " + mark(OrganObservability) + "\n")
	sb.WriteString("              │\n")
	sb.WriteString("              ▼\n")
	sb.WriteString("      EVALUATION " + mark(OrganEvaluation) + "\n")
	sb.WriteString("              │\n")
	sb.WriteString("              ▼\n")
	sb.WriteString("    EVOLUTION + REPRODUCTION " + mark(OrganReproductive) + "\n")
	sb.WriteString("              │\n")
	sb.WriteString("              ▼\n")
	sb.WriteString("      AUTOPOIESIS " + mark(OrganAutopoietic) + "\n")
	sb.WriteString("              │\n")
	sb.WriteString("              ▼\n")
	sb.WriteString("     UPDATED ECOSYSTEM\n")
	return sb.String()
}
