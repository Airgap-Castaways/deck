package askcontext

import "strings"

func AuthoringDocBlock() string {
	b := &strings.Builder{}
	b.WriteString("## Ask authoring context\n\n")
	b.WriteString("- ")
	b.WriteString(Current().Workflow.Summary)
	b.WriteString("\n- ")
	b.WriteString(Current().Components.ImportRule)
	b.WriteString("\n- ")
	b.WriteString(Current().Vars.Summary)
	b.WriteString("\n- Prefer typed steps over `Command` when a typed step exists.\n")
	return b.String()
}

func CLIDocBlock() string {
	b := &strings.Builder{}
	b.WriteString("## Ask CLI context\n\n")
	b.WriteString("- `")
	b.WriteString(Current().CLI.Command)
	b.WriteString("` previews by default; add `--write` to write workflow files.\n")
	b.WriteString("- `")
	b.WriteString(Current().CLI.PlanSubcommand)
	b.WriteString("` saves reusable plan artifacts under `./.deck/plan/`.\n")
	return b.String()
}
