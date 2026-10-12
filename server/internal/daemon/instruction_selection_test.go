package daemon

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func selectionTestFile(t *testing.T, root, name, content string) string {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func selectionTestLink(t *testing.T, target, path string) {
	t.Helper()
	if err := os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}
}

func TestInstructionSelectionReader(t *testing.T) {
	for _, scenario := range []string{"hidden and binary", "git ignored and untracked", "outside git", "explicit hidden and ignored", "links and loop", "duplicates", "empty folder", "binary boundary"} {
		t.Run(scenario, func(t *testing.T) {
			root := t.TempDir()
			var paths, want []string
			switch scenario {
			case "hidden and binary":
				selectionTestFile(t, root, "SKILL.md", "skill")
				selectionTestFile(t, root, ".env", "secret")
				selectionTestFile(t, root, ".git/config", "hidden")
				selectionTestFile(t, root, "logo.png", "png\x00data")
				paths, want = []string{root}, []string{filepath.Join(root, "SKILL.md")}
			case "git ignored and untracked":
				if out, err := exec.Command("git", "-C", root, "init", "-q").CombinedOutput(); err != nil {
					t.Fatalf("git init: %s: %v", out, err)
				}
				selectionTestFile(t, root, ".gitignore", "out/\ntracked.md\n")
				selectionTestFile(t, root, "tracked.md", "tracked")
				if out, err := exec.Command("git", "-C", root, "add", "-f", "tracked.md").CombinedOutput(); err != nil {
					t.Fatalf("git add: %s: %v", out, err)
				}
				for _, name := range []string{" leading.md", "SKILL.md", "notes.md", "out/build.md"} {
					selectionTestFile(t, root, name, name)
				}
				paths = []string{root}
				for _, name := range []string{" leading.md", "SKILL.md", "notes.md", "tracked.md"} {
					want = append(want, filepath.Join(root, name))
				}
			case "outside git":
				selectionTestFile(t, root, ".gitignore", "draft.md\n")
				want = []string{selectionTestFile(t, root, "draft.md", "draft")}
				paths = []string{root}
			case "explicit hidden and ignored":
				if out, err := exec.Command("git", "-C", root, "init", "-q").CombinedOutput(); err != nil {
					t.Fatalf("git init: %s: %v", out, err)
				}
				selectionTestFile(t, root, ".gitignore", "ignored.md\n")
				want = []string{selectionTestFile(t, root, ".omp/agent/skills/persona/SKILL.md", "skill"), selectionTestFile(t, root, ".notes.md", "hidden"), selectionTestFile(t, root, "ignored.md", "ignored")}
				paths = []string{filepath.Dir(want[0]), want[1], want[2]}
			case "links and loop":
				target := t.TempDir()
				selectionTestFile(t, target, "a.md", "a")
				selectionTestLink(t, target, filepath.Join(target, "loop"))
				selectionTestLink(t, target, filepath.Join(root, "folder"))
				selectionTestLink(t, filepath.Join(target, "a.md"), filepath.Join(root, "linked.md"))
				selectionTestLink(t, root, filepath.Join(root, "loop"))
				paths = []string{filepath.Join(root, "folder"), root, filepath.Join(root, "linked.md")}
				want = []string{filepath.Join(root, "folder/a.md")}
			case "duplicates":
				file := selectionTestFile(t, root, "SKILL.md", "skill")
				selectionTestLink(t, file, filepath.Join(root, "z-link.md"))
				paths, want = []string{root, file, root}, []string{file}
			case "empty folder":
				selectionTestFile(t, root, ".DS_Store", "hidden")
				selectionTestFile(t, root, "logo.png", "\x00png")
				paths = []string{root}
			case "binary boundary":
				want = []string{selectionTestFile(t, root, "later.md", strings.Repeat("a", 8192)+"\x00")}
				selectionTestFile(t, root, "binary.md", strings.Repeat("a", 8191)+"\x00")
				paths = []string{root}
			}
			r, err := readInstructionSelection(context.Background(), paths)
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, file := range r.files {
				got = append(got, file.path)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("files = %v, want %v", got, want)
			}
			if scenario == "empty folder" && (len(r.trees) != 1 || !reflect.DeepEqual(r.trees[0].lines, []string{filepath.Base(root) + "/"})) {
				t.Fatalf("empty tree = %+v", r.trees)
			}
		})
	}
}

func TestInstructionSelectionReaderErrors(t *testing.T) {
	for _, scenario := range []string{"missing", "binary", "unreadable file", "unreadable folder"} {
		t.Run(scenario, func(t *testing.T) {
			root := t.TempDir()
			path, wantCause := filepath.Join(root, "gone.md"), "does not exist"
			selected := path
			switch scenario {
			case "binary":
				path = selectionTestFile(t, root, "logo.png", "\x00png")
				selected, wantCause = path, "not a text file"
			case "unreadable file", "unreadable folder":
				if os.Geteuid() == 0 {
					t.Skip("root bypasses filesystem permissions")
				}
				path = selectionTestFile(t, root, "nested/file.md", "text")
				if scenario == "unreadable folder" {
					path = filepath.Dir(path)
				}
				if err := os.Chmod(path, 0000); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = os.Chmod(path, 0700) })
				selected, wantCause = root, "permission denied"
			}
			_, err := readInstructionSelection(context.Background(), []string{selected})
			var selectionErr *instructionSelectionError
			if !errors.As(err, &selectionErr) || selectionErr.path != path || selectionErr.cause != wantCause {
				t.Fatalf("error = %v, want %s: %s", err, path, wantCause)
			}
		})
	}
}

func TestInstructionSelectionExpansion(t *testing.T) {
	root := t.TempDir()
	skill := filepath.Join(root, "as-ux-designer")
	skillFile := selectionTestFile(t, skill, "SKILL.md", "# Skill\n")
	guide := selectionTestFile(t, skill, "references/guide.md", "code & <tag> \"quote\" </file>")
	note := selectionTestFile(t, root, "b.md", "note")
	want := "<selection>\n<tree root=\"" + skill + "\">\nas-ux-designer/\n  SKILL.md\n  references/\n    guide.md\n</tree>\n" +
		"<file path=\"" + skillFile + "\">\n# Skill\n</file>\n" +
		"<file path=\"" + guide + "\">\ncode & <tag> \"quote\" </file>\n</file>\n" +
		"<file path=\"" + note + "\">\nnote\n</file>\n</selection>"
	got, err := expandInstructionSelection(context.Background(), "{selection}", []string{skill, note})
	if err != nil || got != want {
		t.Fatalf("expansion = %q, %v; want %q", got, err, want)
	}
	got, err = expandInstructionSelection(context.Background(), "Read this first.\n\n{selection}\nAgain: {selection}", []string{skill, note})
	if err != nil || got != "Read this first.\n\n"+want+"\nAgain: "+want {
		t.Fatalf("repeated expansion = %q, %v", got, err)
	}
	got, err = expandInstructionSelection(context.Background(), "before{selection}after{selection}", nil)
	if err != nil || got != "beforeafter" {
		t.Fatalf("empty expansion = %q, %v", got, err)
	}
	got, err = expandInstructionSelection(context.Background(), "No placeholder.", []string{filepath.Join(root, "missing.md")})
	if err != nil || got != "No placeholder." {
		t.Fatalf("untouched instructions = %q, %v", got, err)
	}
	if err := os.WriteFile(note, []byte("edited without a sync"), 0600); err != nil {
		t.Fatal(err)
	}
	got, err = expandInstructionSelection(context.Background(), "{selection}", []string{note})
	if err != nil || !strings.Contains(got, "edited without a sync") || strings.Contains(got, "\nnote\n") {
		t.Fatalf("live edit expansion = %q, %v", got, err)
	}
}

func TestInstructionSelectionEscapingAndEmptyTree(t *testing.T) {
	root := t.TempDir()
	folder := filepath.Join(root, "skill&<\"")
	file := selectionTestFile(t, folder, "notes&<\".md", "content & <unchanged>")
	got, err := expandInstructionSelection(context.Background(), "{selection}", []string{folder})
	escapedFolder := strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(folder, "&", "&amp;"), "<", "&lt;"), "\"", "&quot;")
	escapedFile := escapedFolder + "/notes&amp;&lt;&quot;.md"
	want := "<selection>\n<tree root=\"" + escapedFolder + "\">\nskill&<\"/\n  notes&<\".md\n</tree>\n<file path=\"" + escapedFile + "\">\ncontent & <unchanged>\n</file>\n</selection>"
	if err != nil || got != want {
		t.Fatalf("escaped expansion = %q, %v; want %q (file %s)", got, err, want, file)
	}
	empty := filepath.Join(root, "empty")
	if err := os.Mkdir(empty, 0700); err != nil {
		t.Fatal(err)
	}
	selectionTestFile(t, empty, "logo.png", "\x00binary")
	selectionTestFile(t, empty, ".DS_Store", "hidden")
	got, err = expandInstructionSelection(context.Background(), "{selection}", []string{empty})
	want = "<selection>\n<tree root=\"" + empty + "\">\nempty/\n</tree>\n</selection>"
	if err != nil || got != want {
		t.Fatalf("empty tree expansion = %q, %v; want %q", got, err, want)
	}
}
