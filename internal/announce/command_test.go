package announce

import "testing"

func TestParse(t *testing.T) {
	allowed := map[string]struct{}{"legion": {}, "acer": {}}

	got, err := Parse("legion Dinner is ready", allowed)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if got.Target != "legion" || got.Message != "Dinner is ready" {
		t.Fatalf("unexpected command: %+v", got)
	}
}

func TestParseAll(t *testing.T) {
	allowed := map[string]struct{}{"legion": {}, "acer": {}}

	got, err := Parse("all Testing one two", allowed)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if got.Target != "all" || got.Message != "Testing one two" {
		t.Fatalf("unexpected command: %+v", got)
	}
}

func TestParseRejectsUnknownTarget(t *testing.T) {
	allowed := map[string]struct{}{"legion": {}, "acer": {}}
	if _, err := Parse("desktop Hello", allowed); err == nil {
		t.Fatal("expected unknown target error")
	}
}

func TestParseRequiresMessage(t *testing.T) {
	allowed := map[string]struct{}{"legion": {}}
	if _, err := Parse("legion", allowed); err == nil {
		t.Fatal("expected usage error")
	}
}
