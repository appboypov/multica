package cli

import (
	"os"
	"testing"

	"github.com/multica-ai/multica/server/internal/testutil/testenv"
)

// TestMain runs the package without daemon task identity, so profile paths
// resolve from HOME unless a test sets TaskConfigRootEnv itself.
func TestMain(m *testing.M) {
	testenv.ClearDaemonTaskEnv()
	os.Exit(m.Run())
}
