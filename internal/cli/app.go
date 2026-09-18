package cli

import (
	"errors"
	"fmt"
)

// Run is the application boundary used by cmd/pplsite. It intentionally
// returns errors so process exit policy remains in the command entrypoint.
func Run(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: pplsite <validate|build|prepare|verify-handoff|release-candidates|release-record|publish-plan>")
	}
	switch args[0] {
	case "validate":
		return validateCommand(args[1:])
	case "build":
		return buildCommand(args[1:])
	case "prepare":
		return prepareCommand(args[1:])
	case "verify-handoff":
		return verifyHandoffCommand(args[1:])
	case "release-candidates":
		return releaseCandidatesCommand(args[1:])
	case "release-record":
		return releaseRecordCommand(args[1:])
	case "publish-plan":
		return publishPlanCommand(args[1:])
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}
