package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

type fakeParser struct{ name string }

func (f *fakeParser) Name() string                   { return f.name }
func (f *fakeParser) Discover() ([]string, error)    { return nil, nil }
func (f *fakeParser) Parse(string) ([]Record, error) { return nil, nil }

func TestAllReturnsSortedNames(t *testing.T) {
	names := []string{}
	for _, p := range All() {
		names = append(names, p.Name())
	}
	for i := 1; i < len(names); i++ {
		if names[i-1] > names[i] {
			t.Fatalf("All() not sorted: %v", names)
		}
	}
	// The three built-in parsers must be registered via their init() functions.
	for _, want := range []string{"claude", "codex", "opencode"} {
		if _, err := Get(want); err != nil {
			t.Errorf("expected built-in parser %q to be registered: %v", want, err)
		}
	}
}

func TestRegisterAndGet(t *testing.T) {
	Register(&fakeParser{name: "zzz-test"})
	p, err := Get("zzz-test")
	if err != nil {
		t.Fatalf("Get after Register: %v", err)
	}
	if p.Name() != "zzz-test" {
		t.Errorf("got %q, want zzz-test", p.Name())
	}
	if _, err := Get("does-not-exist"); err == nil {
		t.Error("expected error for unknown agent")
	}
}

func TestSortStrings(t *testing.T) {
	s := []string{"opencode", "claude", "codex"}
	sortStrings(s)
	want := []string{"claude", "codex", "opencode"}
	if !reflect.DeepEqual(s, want) {
		t.Errorf("got %v, want %v", s, want)
	}
}

func TestRootsFromEnv(t *testing.T) {
	const name = "CLAUDE_COST_TEST_DIRS"
	fallback := []string{"/default/a", "/default/b"}

	t.Run("unset returns fallback", func(t *testing.T) {
		if err := os.Unsetenv(name); err != nil {
			t.Fatal(err)
		}
		if got := rootsFromEnv(name, fallback); !reflect.DeepEqual(got, fallback) {
			t.Errorf("got %v, want fallback %v", got, fallback)
		}
	})

	t.Run("set returns split list", func(t *testing.T) {
		t.Setenv(name, "/one"+string(os.PathListSeparator)+"/two")
		got := rootsFromEnv(name, fallback)
		want := []string{"/one", "/two"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("empty returns fallback", func(t *testing.T) {
		t.Setenv(name, "")
		if got := rootsFromEnv(name, fallback); !reflect.DeepEqual(got, fallback) {
			t.Errorf("got %v, want fallback %v", got, fallback)
		}
	})
}

func TestDiscover(t *testing.T) {
	root := t.TempDir()
	want := filepath.Join(root, "proj", "a.jsonl")
	if err := os.MkdirAll(filepath.Dir(want), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(want, []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	// A file that should not match the tail pattern.
	if err := os.WriteFile(filepath.Join(root, "proj", "b.txt"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := discover([]string{root, "/nonexistent/root"}, filepath.Join("*", "*.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != want {
		t.Errorf("got %v, want [%s]", got, want)
	}
}
