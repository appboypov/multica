package daemon

import (
	"fmt"
	"os"
	"testing"

	"github.com/multica-ai/multica/server/internal/testutil/testenv"
)

// TestMain runs the package without daemon task identity and with an empty
// global git config, so git fixtures depend only on the settings they write.
func TestMain(m *testing.M) {
	testenv.ClearDaemonTaskEnv()
	cleanup, err := testenv.IsolateGitConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	code := m.Run()
	cleanup()
	os.Exit(code)
}
