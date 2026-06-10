package tools

import (
	"os"
	"strings"
	"testing"

	"github.com/diskfs/go-diskfs/backend/file"
	"github.com/diskfs/go-diskfs/filesystem/iso9660"
)

func TestAnswerLayout(t *testing.T) {
	cases := map[string]struct {
		label, file string
		ok          bool
	}{
		"kickstart":    {"OEMDRV", "ks.cfg", true},
		"NoCloud":      {"CIDATA", "user-data", true},
		"autounattend": {"ANSWER", "autounattend.xml", true},
		"preseed":      {"", "", false},
	}
	for kind, want := range cases {
		label, fname, ok := answerLayout(kind)
		if ok != want.ok || label != want.label || fname != want.file {
			t.Errorf("answerLayout(%q) = (%q, %q, %v), want (%q, %q, %v)",
				kind, label, fname, ok, want.label, want.file, want.ok)
		}
	}
}

// TestBuildAnswerISO_PreservesExactNames is the load-bearing test: NoCloud and
// Windows fail silently if the answer filenames get 8.3-mangled, so we round-trip
// the ISO and assert the exact, case-sensitive names and contents survive.
func TestBuildAnswerISO_PreservesExactNames(t *testing.T) {
	want := map[string]string{
		"user-data": "#cloud-config\nhostname: test\n",
		"meta-data": "instance-id: iid-local01\n",
	}
	in := map[string][]byte{}
	for k, v := range want {
		in[k] = []byte(v)
	}

	data, err := buildAnswerISO("CIDATA", in)
	if err != nil {
		t.Fatalf("buildAnswerISO: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("ISO data is empty")
	}

	tmp, err := os.CreateTemp(t.TempDir(), "read-*.iso")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tmp.Write(data); err != nil {
		t.Fatal(err)
	}
	if _, err := tmp.Seek(0, 0); err != nil {
		t.Fatal(err)
	}

	bk := file.New(tmp, true)
	fs, err := iso9660.Read(bk, 0, 0, 2048)
	if err != nil {
		t.Fatalf("reading ISO back: %v", err)
	}

	if got := strings.TrimRight(fs.Label(), "\x00 "); got != "CIDATA" {
		t.Errorf("volume label = %q, want CIDATA", got)
	}

	for name, content := range want {
		got, err := fs.ReadFile(name)
		if err != nil {
			t.Errorf("ReadFile(%q): %v (filename may have been mangled)", name, err)
			continue
		}
		if string(got) != content {
			t.Errorf("content of %q = %q, want %q", name, got, content)
		}
	}
}
