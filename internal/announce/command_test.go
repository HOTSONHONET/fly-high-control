package announce

import "testing"

func TestParse(t *testing.T) {
	allowed := map[string]struct{}{"legion": {}, "acer": {}}

	got, err := Parse("3 legion Dinner is ready", allowed)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if got.Repeat != 3 || got.Target != "legion" || got.Message != "Dinner is ready" {
		t.Fatalf("unexpected command: %+v", got)
	}
}

func TestParseAll(t *testing.T) {
	allowed := map[string]struct{}{"legion": {}, "acer": {}}

	got, err := Parse("2 all Testing one two", allowed)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if got.Repeat != 2 || got.Target != "all" || got.Message != "Testing one two" {
		t.Fatalf("unexpected command: %+v", got)
	}
}

func TestParseRejectsUnknownTarget(t *testing.T) {
	allowed := map[string]struct{}{"legion": {}, "acer": {}}
	if _, err := Parse("1 desktop Hello", allowed); err == nil {
		t.Fatal("expected unknown target error")
	}
}

func TestParseRequiresMessage(t *testing.T) {
	allowed := map[string]struct{}{"legion": {}}
	if _, err := Parse("1 legion", allowed); err == nil {
		t.Fatal("expected usage error")
	}
}

func TestParseRejectsInvalidRepeat(t *testing.T) {
	allowed := map[string]struct{}{"legion": {}}

	for _, input := range []string{
		"zero legion Hello",
		"0 legion Hello",
		"11 legion Hello",
	} {
		if _, err := Parse(input, allowed); err == nil {
			t.Fatalf("expected repeat validation error for %q", input)
		}
	}
}
