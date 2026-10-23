package cli

import (
	"os"
	"path/filepath"
	"strings"
)

// ProfileFile is the file that names, in one line, the CLI profile for its
// folder and every folder below it, so a command run there acts as that
// profile's login without --profile or MULTICA_* variables.
const ProfileFile = ".multica-profile"

// FolderProfile returns the profile the nearest ProfileFile in folder or a
// folder above it names, with surrounding whitespace trimmed; the folder's own
// file wins over one above it. A blank or unreadable file names no profile,
// so the search goes on above it. Without a named profile it returns "", the
// default profile (~/.multica/config.json).
func FolderProfile(folder string) string {
	current, err := filepath.Abs(folder)
	if err != nil {
		return ""
	}
	for {
		if data, err := os.ReadFile(filepath.Join(current, ProfileFile)); err == nil {
			if name := strings.TrimSpace(string(data)); name != "" {
				return name
			}
		}
		parent := filepath.Dir(current)
		if parent == current {
			return ""
		}
		current = parent
	}
}
