package tool

import "strings"

// DangerousCommands that require user confirmation
var dangerousPatterns = []string{
	"rm ", "rm\t", "rmdir", "del ",
	"sudo ", "format ",
	"git push", "git reset --hard", "git clean",
	"shutdown", "reboot",
	"DROP ", "DELETE FROM", "TRUNCATE ",
	"mkfs", "dd if=",
	"> /dev/", "chmod 777",
}

// IsDangerousCommand checks if a command needs approval
func IsDangerousCommand(command string) bool {
	lower := strings.ToLower(command)
	for _, p := range dangerousPatterns {
		if strings.Contains(lower, strings.ToLower(p)) {
			return true
		}
	}
	return false
}

// ClassifyRisk returns a risk level for a command
func ClassifyRisk(command string) string {
	if IsDangerousCommand(command) {
		return "dangerous"
	}
	lower := strings.ToLower(command)
	writePatterns := []string{"mv ", "cp ", "mkdir ", "touch ", "echo ", "cat >", "tee "}
	for _, p := range writePatterns {
		if strings.Contains(lower, p) {
			return "write"
		}
	}
	return "safe"
}
