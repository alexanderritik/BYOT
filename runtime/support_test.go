package runtime

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStageSupportFiles_Locust(t *testing.T) {
	ws := t.TempDir()
	if err := StageSupportFiles("locust", ws); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"run.sh", "byot_checks.py"} {
		info, err := os.Stat(filepath.Join(ws, SupportDir, name))
		if err != nil {
			t.Fatalf("expected %s staged: %v", name, err)
		}
		if info.Size() == 0 {
			t.Fatalf("%s is empty", name)
		}
	}
}

func TestStageSupportFiles_OtherRuntimesNoop(t *testing.T) {
	ws := t.TempDir()
	if err := StageSupportFiles("python", ws); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(ws, SupportDir)); !os.IsNotExist(err) {
		t.Fatalf("expected no %s dir for python, got err=%v", SupportDir, err)
	}
}

func TestIsBundle(t *testing.T) {
	cases := []struct {
		runtime, file string
		want          bool
	}{
		{"playwright", "suite.zip", true},
		{"locust", "load.zip", true},
		{"locust", "LOAD.ZIP", true},
		{"locust", "locustfile.py", false},
		{"python", "script.zip", false},
		{"k6", "test.js", false},
	}
	for _, c := range cases {
		if got := IsBundle(c.runtime, c.file); got != c.want {
			t.Errorf("IsBundle(%q, %q) = %v, want %v", c.runtime, c.file, got, c.want)
		}
	}
}
