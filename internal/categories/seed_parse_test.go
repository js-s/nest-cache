package categories

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"
)

// Drift guard: the embedded mirror must parse identically to
// resources/categories.yaml. Sync the mirror when this fails.
func TestSeedMirrorMatchesSource(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "resources", "categories.yaml"))
	if err != nil {
		t.Fatalf("read source yaml: %v", err)
	}
	var want, got seedDoc
	if err := yaml.Unmarshal(raw, &want); err != nil {
		t.Fatalf("parse source yaml: %v", err)
	}
	if err := yaml.Unmarshal(seedMirrored, &got); err != nil {
		t.Fatalf("parse embedded mirror: %v", err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Fatal("seed mirror drifted from resources/categories.yaml; copy the source over seed_categories.yaml")
	}
}

func TestParseSeed(t *testing.T) {
	groups, err := parseSeed(seedMirrored)
	if err != nil {
		t.Fatalf("parseSeed: %v", err)
	}
	var expense, income, kids int
	seen := map[string]bool{}
	for _, g := range groups {
		if g.name == "" || seen[g.kind+"/"+g.name] {
			t.Fatalf("bad or duplicate group: %+v", g)
		}
		seen[g.kind+"/"+g.name] = true
		switch g.kind {
		case KindExpense:
			expense++
			kids += len(g.children)
		case KindIncome:
			income++
			if len(g.children) != 0 {
				t.Fatalf("income group %q should be childless, got %v", g.name, g.children)
			}
		default:
			t.Fatalf("unknown kind %q", g.kind)
		}
	}
	if expense != 9 || income != 5 {
		t.Fatalf("groups = %d expense + %d income, want 9 + 5", expense, income)
	}
	if kids != 27 {
		t.Fatalf("expense children = %d, want 27", kids)
	}
}

func TestHumanize(t *testing.T) {
	for slug, want := range map[string]string{
		"codzienne": "Codzienne", "styl_zycia": "Styl życia", "pozostale": "Pozostałe",
		"mieszkanie": "Mieszkanie",
	} {
		if got := humanize(slug); got != want {
			t.Errorf("humanize(%q) = %q, want %q", slug, got, want)
		}
	}
}
