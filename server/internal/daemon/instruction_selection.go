package daemon

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const failureReasonInstructionSelection = "instruction_selection_error"

type instructionSelectionError struct {
	path  string
	cause string
}

func (e *instructionSelectionError) Error() string {
	return fmt.Sprintf("instruction selection: %s: %s", e.path, e.cause)
}

func selectionReadError(path string, err error) error {
	cause := err.Error()
	switch {
	case errors.Is(err, os.ErrNotExist):
		cause = "does not exist"
	case errors.Is(err, os.ErrPermission):
		cause = "permission denied"
	}
	return &instructionSelectionError{path: path, cause: cause}
}

type instructionSelectionFile struct {
	path    string
	content []byte
}

type instructionSelectionTree struct {
	root  string
	lines []string
}

type instructionSelectionReader struct {
	visited map[string]struct{}
	seen    map[string]struct{}
	trees   []instructionSelectionTree
	files   []instructionSelectionFile
}

func readInstructionSelection(ctx context.Context, paths []string) (*instructionSelectionReader, error) {
	r := &instructionSelectionReader{visited: make(map[string]struct{}), seen: make(map[string]struct{})}
	for _, path := range paths {
		real, info, err := resolveSelectionPath(path)
		if err != nil {
			return nil, err
		}
		if info.IsDir() {
			lines, err := r.walk(ctx, path, real, selectionGitFiles(ctx, real), 1)
			if err != nil {
				return nil, err
			}
			r.trees = append(r.trees, instructionSelectionTree{root: path, lines: append([]string{filepath.Base(path) + "/"}, lines...)})
		} else if _, err := r.readFile(path, real, true); err != nil {
			return nil, err
		}
	}
	return r, nil
}

func resolveSelectionPath(path string) (string, os.FileInfo, error) {
	real, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", nil, selectionReadError(path, err)
	}
	info, err := os.Stat(real)
	if err != nil {
		return "", nil, selectionReadError(path, err)
	}
	return real, info, nil
}

func selectionGitFiles(ctx context.Context, root string) map[string]struct{} {
	top, err := runGitCommandContext(ctx, root, 30*time.Second, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil
	}
	out, err := runGitCommandRawContext(ctx, root, 30*time.Second, "ls-files", "--cached", "--others", "--exclude-standard", "-z", "--full-name")
	if err != nil {
		return nil
	}
	allowed := make(map[string]struct{})
	for _, name := range strings.Split(out, "\x00") {
		if name != "" {
			allowed[filepath.Join(top, name)] = struct{}{}
		}
	}
	return allowed
}

func (r *instructionSelectionReader) walk(ctx context.Context, path, real string, allowed map[string]struct{}, depth int) ([]string, error) {
	if _, ok := r.visited[real]; ok {
		return nil, nil
	}
	r.visited[real] = struct{}{}
	entries, err := os.ReadDir(real)
	if err != nil {
		return nil, selectionReadError(path, err)
	}
	var lines []string
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		child := filepath.Join(path, entry.Name())
		childReal, info, err := resolveSelectionPath(filepath.Join(real, entry.Name()))
		if err != nil {
			var readErr *instructionSelectionError
			if errors.As(err, &readErr) {
				readErr.path = child
			}
			return nil, err
		}
		prefix := strings.Repeat("  ", depth)
		if info.IsDir() {
			childAllowed := allowed
			if entry.Type()&os.ModeSymlink != 0 {
				childAllowed = selectionGitFiles(ctx, childReal)
			}
			children, err := r.walk(ctx, child, childReal, childAllowed, depth+1)
			if err != nil {
				return nil, err
			}
			if len(children) > 0 {
				lines = append(lines, prefix+entry.Name()+"/")
				lines = append(lines, children...)
			}
			continue
		}
		if allowed != nil {
			if _, ok := allowed[filepath.Join(real, entry.Name())]; !ok {
				continue
			}
		}
		included, err := r.readFile(child, childReal, false)
		if err != nil {
			return nil, err
		}
		if included {
			lines = append(lines, prefix+entry.Name())
		}
	}
	return lines, nil
}

func (r *instructionSelectionReader) readFile(path, real string, selected bool) (bool, error) {
	if _, ok := r.seen[real]; ok {
		return false, nil
	}
	content, err := os.ReadFile(real)
	if err != nil {
		return false, selectionReadError(path, err)
	}
	if bytes.IndexByte(content[:min(len(content), 8*1024)], 0) >= 0 {
		if selected {
			return false, &instructionSelectionError{path: path, cause: "not a text file"}
		}
		return false, nil
	}
	r.seen[real] = struct{}{}
	r.files = append(r.files, instructionSelectionFile{path: path, content: content})
	return true, nil
}

var selectionAttributeEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", "\"", "&quot;")

func expandInstructionSelection(ctx context.Context, instructions string, paths []string) (string, error) {
	if !strings.Contains(instructions, "{selection}") {
		return instructions, nil
	}
	if len(paths) == 0 {
		return strings.ReplaceAll(instructions, "{selection}", ""), nil
	}
	selection, err := readInstructionSelection(ctx, paths)
	if err != nil {
		return "", err
	}
	var rendered strings.Builder
	rendered.WriteString("<selection>\n")
	for _, tree := range selection.trees {
		rendered.WriteString("<tree root=\"")
		rendered.WriteString(selectionAttributeEscaper.Replace(tree.root))
		rendered.WriteString("\">\n")
		for _, line := range tree.lines {
			rendered.WriteString(line)
			rendered.WriteByte('\n')
		}
		rendered.WriteString("</tree>\n")
	}
	for _, file := range selection.files {
		rendered.WriteString("<file path=\"")
		rendered.WriteString(selectionAttributeEscaper.Replace(file.path))
		rendered.WriteString("\">\n")
		rendered.Write(file.content)
		if len(file.content) == 0 || file.content[len(file.content)-1] != '\n' {
			rendered.WriteByte('\n')
		}
		rendered.WriteString("</file>\n")
	}
	rendered.WriteString("</selection>")
	return strings.ReplaceAll(instructions, "{selection}", rendered.String()), nil
}
