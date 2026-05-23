package loader

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestHTMLLoader_StripsTags(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "x.html")
	_ = os.WriteFile(p, []byte("<html><body><h1>Title</h1><script>evil()</script><p>hello world</p></body></html>"), 0o644)
	d, err := HTMLLoader{}.Load(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(d.Text, "hello world") || contains(d.Text, "evil()") {
		t.Fatalf("unexpected text: %q", d.Text)
	}
}

func TestTextLoader_Passthrough(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "x.txt")
	_ = os.WriteFile(p, []byte("Plain text body."), 0o644)
	d, err := TextLoader{}.Load(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	if d.Text != "Plain text body." {
		t.Fatalf("got %q", d.Text)
	}
}

func TestExtractEntities_FindsBasics(t *testing.T) {
	ents := ExtractEntities("On 2026-01-12 Swiggy charged ₹450 from INR account 411111111111.")
	saw := map[string]bool{}
	for _, e := range ents {
		saw[e.Type] = true
	}
	for _, expect := range []string{"date", "amount", "currency", "account", "merchant"} {
		if !saw[expect] {
			t.Errorf("expected entity type %s, all=%+v", expect, ents)
		}
	}
}

func contains(s, sub string) bool { return len(s) >= len(sub) && indexOf(s, sub) >= 0 }
func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
