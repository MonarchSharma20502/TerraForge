// Package analyze: the recommendation ruleset. Each rule is self-contained: it
// declares an id, a severity and a check, and reads either the graph (typed
// blocks) or the raw file bytes (for substring rules that must also catch
// comments). Rules never panic on input they do not understand.
package analyze

// defaultRules is the ruleset the analyzer runs, ordered by how fundamental each
// rule is. The order only affects the display of same-severity findings.
func defaultRules(c *Classifier) []Rule {
	return []Rule{
		noHardcodedSecrets{},
		mandatoryTags{c: c},
		noPublicIngress{c: c},
		namingConformance{c: c},
		noInsecureTransport{},
		noVMPublicIP{},
		noVMWithoutAvailability{},
		noBackup{c: c},
		noDiagnosticLogging{c: c},
		noResourceLocks{},
		unpinnedProviderVersion{},
		unpinnedModuleSource{},
		hardcodedLocation{},
	}
}

// ruleBase gives every rule a stable id and severity.
type ruleBase struct {
	name string
	sev  Severity
}

func (r ruleBase) ID() string    { return r.name }
func (r ruleBase) severity() Severity { return r.sev }

// finding is the shorthand every rule uses to build one recommendation.
func finding(rule string, sev Severity, message, fix string, files ...string) Finding {
	return Finding{
		Rule:     rule,
		Severity: sev,
		Message:  message,
		Fix:      fix,
		Files:    files,
	}
}
