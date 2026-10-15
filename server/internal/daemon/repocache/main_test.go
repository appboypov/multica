package repocache

import (
	"fmt"
	"os"
	"testing"

	"github.com/multica-ai/multica/server/internal/testutil/testenv"
)

// TestMain runs the package with an empty global git config, so git fixtures
// depend only on the settings they write.
func TestMain(m *testing.M) {
	cleanup, err := testenv.IsolateGitConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	code := m.Run()
	cleanup()
	os.Exit(code)
}
