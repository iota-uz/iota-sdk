package configurator

import (
	"context"

	"github.com/iota-uz/iota-sdk/pkg/intl"
)

// messagePrefix is the locale namespace of the screen's strings.
const messagePrefix = "Extensions.Configurator."

// MessageKeys are the locale keys (below Extensions.Configurator.) the screen
// reads through the host's locale service. Every key exists in every locale
// file of the core module.
var MessageKeys = []string{
	"Action", "Activate", "Active", "Actor", "Approve", "Approved", "ArtifactHash", "AssignOwner", "Audit",
	"Bind", "Bindings", "Build", "Cancel", "Check", "Close", "Comment", "Commit", "Conflict", "ConflictText",
	"Create", "Deactivate", "Decide", "Decision", "Delete", "DeleteConfirm", "Details", "Editor",
	"EditorLoadFailed", "Export", "Files", "Filter", "Hashes", "History", "Import", "ImportGit", "KeepMine",
	"KitHash", "Loading", "Namespace", "NewFile", "NewFilePath", "NewPackage", "NoAudit", "NoPackages",
	"NoProblems", "NoVersions", "NoWorkflows", "NotApproved", "Occurrences", "Outcome", "Output", "Owners",
	"Packages", "Payload", "Problems", "Publish", "ReadOnly", "Refresh", "Reject", "Remove", "Rename", "RepoURL",
	"Resolve", "Resume", "Retry", "RetryStep", "Save", "SaveAll", "SelectFile", "SelectPackage", "Signal",
	"SignalName", "Signals", "SlotKind", "SourceHash", "StaleProblems", "StartV2", "Started", "Status", "Subdir",
	"TakeTheirs", "TargetID", "Tasks", "Terminate", "Test", "TestsFailed", "TestsPassed", "Title", "TriggerID",
	"TypesLoadFailed", "Unbind", "Unsaved", "UserID", "Version", "Versions", "Workflow", "Workflows", "Working",
}

// Messages resolves the screen's strings for the request locale. A key that
// has no translation falls back to the key itself.
func Messages(ctx context.Context) map[string]string {
	out := make(map[string]string, len(MessageKeys))
	for _, k := range MessageKeys {
		out[messagePrefix+k] = intl.TWithDefault(ctx, messagePrefix+k, k)
	}
	return out
}
