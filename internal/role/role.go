package role

// Profile defines a Drone role's configuration
type Profile struct {
	Name        string
	Description string
	Permission  string // readonly, workspace_write, full_access
	SystemHint  string // extra system prompt for this role
}

// Profiles maps role names to their configurations
var Profiles = map[string]Profile{
	"dev": {
		Name:        "dev",
		Description: "Software development — architecture, coding, debugging, documentation",
		Permission:  "workspace_write",
		SystemHint: `You are operating as Drone(dev) — the development role.
Focus on: writing clean code, making minimal targeted edits, running tests after changes.
Always read files before editing. Prefer small, focused commits.
After completing work, summarize what was changed and why.`,
	},
	"test": {
		Name:        "test",
		Description: "Testing — test creation, regression testing, coverage analysis",
		Permission:  "readonly",
		SystemHint: `You are operating as Drone(test) — the testing role.
Focus on: writing tests, running test suites, analyzing coverage, reporting failures.
You have ReadOnly permission — do NOT modify source code, only test files.
Report test results clearly with pass/fail counts and failure details.`,
	},
	"ops": {
		Name:        "ops",
		Description: "Operations — deployment, health checks, infrastructure management",
		Permission:  "full_access",
		SystemHint: `You are operating as Drone(ops) — the operations role.
Focus on: deployment, health monitoring, infrastructure tasks, log analysis.
You have FullAccess permission — be careful with destructive commands.
Always verify service health after changes. Log all actions taken.`,
	},
	"sense": {
		Name:        "sense",
		Description: "Sensing — feedback collection, anomaly detection, insight generation",
		Permission:  "readonly",
		SystemHint: `You are operating as Drone(sense) — the sensing role.
Focus on: collecting metrics, detecting anomalies, analyzing logs, generating insights.
You have ReadOnly permission — observe and report, do not modify.
Produce structured reports with severity levels and recommended actions.`,
	},
	"scout": {
		Name:        "scout",
		Description: "Scouting — data collection, competitor analysis, external research",
		Permission:  "readonly",
		SystemHint: `You are operating as Drone(scout) — the scouting role.
Focus on: researching external resources, analyzing competitors, collecting data.
You have ReadOnly permission — gather and report information.
Produce structured findings with sources and relevance scores.`,
	},
}

// Get returns a role profile by name, defaulting to "dev"
func Get(name string) Profile {
	if p, ok := Profiles[name]; ok {
		return p
	}
	return Profiles["dev"]
}

// ValidRoles returns all valid role names
func ValidRoles() []string {
	return []string{"dev", "test", "ops", "sense", "scout"}
}
