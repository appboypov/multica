package daemon

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestDirectoryListingOMPRoots(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	providerRoot := filepath.Join(home, ".omp", "agent", "skills")
	if err := os.MkdirAll(providerRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	resolved, entries, err := listRuntimeDirectory("omp", "")
	want := []directoryEntry{{Name: "~/.omp/agent/skills", Path: providerRoot, Kind: "folder"}}
	if err != nil || resolved != "" || !reflect.DeepEqual(entries, want) {
		t.Fatalf("omp roots = %q, %#v, %v; want %#v", resolved, entries, err, want)
	}
	universalRoot := filepath.Join(home, ".agents", "skills")
	if err := os.MkdirAll(universalRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	want = append([]directoryEntry{{Name: "~/.agents/skills", Path: universalRoot, Kind: "folder"}}, want...)
	_, entries, err = listRuntimeDirectory("omp", "")
	if err != nil || !reflect.DeepEqual(entries, want) {
		t.Fatalf("all roots = %#v, %v; want %#v", entries, err, want)
	}
	if err := os.RemoveAll(providerRoot); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(universalRoot); err != nil {
		t.Fatal(err)
	}
	_, entries, err = listRuntimeDirectory("omp", "")
	if err != nil || entries == nil || len(entries) != 0 {
		t.Fatalf("no roots = %#v, %v", entries, err)
	}
}

func TestDirectoryListingFolderAndHomePaths(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	folder := filepath.Join(home, "notes")
	for _, name := range []string{".hidden", "folder"} {
		if err := os.MkdirAll(filepath.Join(folder, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(folder, "README.md"), []byte("not sent in a listing"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(folder, "folder"), filepath.Join(folder, "link")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(folder, "missing"), filepath.Join(folder, "broken")); err != nil {
		t.Fatal(err)
	}
	want := []directoryEntry{
		{Name: ".hidden", Path: filepath.Join(folder, ".hidden"), Kind: "folder", Hidden: true},
		{Name: "README.md", Path: filepath.Join(folder, "README.md"), Kind: "file"},
		{Name: "broken", Path: filepath.Join(folder, "broken"), Kind: "file"},
		{Name: "folder", Path: filepath.Join(folder, "folder"), Kind: "folder"},
		{Name: "link", Path: filepath.Join(folder, "link"), Kind: "folder"},
	}
	for _, path := range []string{folder, "~/notes"} {
		resolved, entries, err := listRuntimeDirectory("omp", path)
		if err != nil || resolved != folder || !reflect.DeepEqual(entries, want) {
			t.Fatalf("list %q = %q, %#v, %v; want %#v", path, resolved, entries, err, want)
		}
	}
	for _, path := range []string{filepath.Join(folder, "folder"), filepath.Join(folder, "link"), filepath.Join(folder, ".hidden")} {
		resolved, entries, err := listRuntimeDirectory("omp", path)
		if err != nil || resolved != path || entries == nil || len(entries) != 0 {
			t.Fatalf("empty/linked folder %q = %q, %#v, %v", path, resolved, entries, err)
		}
	}
}

func TestDirectoryListingPathErrors(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "file.md")
	if err := os.WriteFile(file, []byte("file"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ path, errorText string }{
		{"notes", "path must be absolute"},
		{filepath.Join(root, "missing"), "path does not exist: " + filepath.Join(root, "missing")},
		{file, "not a folder: " + file},
	} {
		_, _, err := listRuntimeDirectory("omp", tc.path)
		if err == nil || err.Error() != tc.errorText {
			t.Fatalf("list %q error = %v; want %q", tc.path, err, tc.errorText)
		}
	}
}

func TestDirectoryListingHeartbeatActionReports(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	folder := filepath.Join(home, "notes")
	if err := os.Mkdir(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(folder, "a.md"), []byte("local only"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ path, status string }{
		{"~/notes", "completed"},
		{filepath.Join(home, "missing"), "failed"},
	} {
		t.Run(tc.status, func(t *testing.T) {
			type report struct {
				Status  string           `json:"status"`
				Path    string           `json:"path"`
				Entries []directoryEntry `json:"entries"`
				Error   string           `json:"error"`
			}
			results := make(chan report, 1)
			d, _ := localSkillReportDaemon(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != "/api/daemon/runtimes/runtime-a/directory-listings/request-a/result" {
					t.Errorf("report request = %s %s", r.Method, r.URL.Path)
				}
				var body report
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				w.Write([]byte(`{"status":"ok"}`))
				results <- body
			})
			d.runtimeIndex = map[string]Runtime{"runtime-a": {ID: "runtime-a", Provider: "omp"}}
			d.handleHeartbeatActions(context.Background(), "runtime-a", &HeartbeatResponse{PendingDirectoryListing: &PendingDirectoryListing{ID: "request-a", Path: tc.path}})
			select {
			case body := <-results:
				if body.Status != tc.status {
					t.Fatalf("report status = %#v", body)
				}
				if tc.status == "completed" && (body.Path != folder || !reflect.DeepEqual(body.Entries, []directoryEntry{{Name: "a.md", Path: filepath.Join(folder, "a.md"), Kind: "file"}})) {
					t.Fatalf("completed report = %#v", body)
				}
				if tc.status == "failed" && (!strings.Contains(body.Error, tc.path) || !strings.Contains(body.Error, "does not exist")) {
					t.Fatalf("failed report = %#v", body)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("directory-listing heartbeat action did not report")
			}
		})
	}
}

func TestDirectoryListingReportUsesRuntimeRetryPolicy(t *testing.T) {
	withFastLocalSkillReportBackoffs(t)
	for _, tc := range []struct {
		name             string
		status, attempts int
	}{
		{"transient", http.StatusInternalServerError, 3},
		{"permanent", http.StatusNotFound, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var hits int32
			d, calls := localSkillReportDaemon(t, func(w http.ResponseWriter, _ *http.Request) {
				n := atomic.AddInt32(&hits, 1)
				if n < 3 {
					http.Error(w, `{"error":"report rejected"}`, tc.status)
					return
				}
				w.Write([]byte(`{"status":"ok"}`))
			})
			d.handleDirectoryListing(context.Background(), Runtime{ID: "runtime-a", Provider: "omp"}, "request-a", t.TempDir())
			if got := atomic.LoadInt32(calls); int(got) != tc.attempts {
				t.Fatalf("report attempts = %d; want %d", got, tc.attempts)
			}
		})
	}
}
