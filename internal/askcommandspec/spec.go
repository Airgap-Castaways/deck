package askcommandspec

type Flag struct {
	Name        string
	Description string
}

type Command struct {
	Use   string
	Short string
	Long  string
	Flags []Flag
}

type Spec struct {
	Root   Command
	Plan   Command
	Config Command
}

func Current() Spec {
	return Spec{
		Root: Command{
			Use:   "ask [request]",
			Short: "(Experimental) AI helper for drafting and reviewing workflows",
			Flags: []Flag{
				{Name: "--create", Description: "Treat the request as new workflow authoring."},
				{Name: "--edit", Description: "Treat the request as workflow refinement."},
				{Name: "--review", Description: "Review the current workspace without writing files."},
				{Name: "--from", Description: "Load additional request details from a text or markdown file."},
				{Name: "--plan-name", Description: "Optional plan artifact name used by ask plan."},
				{Name: "--plan-dir", Description: "Directory for ask plan artifacts."},
			},
		},
		Plan: Command{
			Use:   "plan [request]",
			Short: "Generate an ask plan artifact without writing workflow files",
			Long:  "Generate a reusable planning artifact under .deck/plan without writing workflow files. This mode is intended for draft/refine style authoring requests.",
			Flags: []Flag{
				{Name: "--from", Description: "Load additional request details from a text or markdown file."},
				{Name: "--plan-name", Description: "Optional plan artifact name."},
				{Name: "--plan-dir", Description: "Directory for ask plan artifacts."},
			},
		},
		Config: Command{Use: "config", Short: "Manage global ask config defaults and API credentials"},
	}
}
