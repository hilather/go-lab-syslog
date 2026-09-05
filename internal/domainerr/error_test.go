package domainerr

import "testing"

func TestCatalogSeed(t *testing.T) {
	for _, c := range Codes() {
		info := Lookup(c)
		if info.Code != c || info.Status == 0 || info.Title == "" {
			t.Fatalf("%s: %+v", c, info)
		}
		if TypeURI(c) != "https://labsyslog.dev/errors/"+string(c) {
			t.Fatalf("type %s", TypeURI(c))
		}
		if URN(c) != "urn:labsyslog:error:"+string(c) {
			t.Fatalf("urn %s", URN(c))
		}
	}
	if !Is(New(TLSUnsupported, "x"), TLSUnsupported) {
		t.Fatal("Is")
	}
}

func TestErrorString(t *testing.T) {
	err := New(UnknownField, "spec.foo")
	if err.Error() != "unknown_field: spec.foo" {
		t.Fatalf("%q", err.Error())
	}
}

func TestProblemOf(t *testing.T) {
	p := ProblemOf(New(WaitTimeout, "wait timed out"))
	if p.Type != "https://labsyslog.dev/errors/wait_timeout" || p.Status != 504 || p.Code != WaitTimeout {
		t.Fatalf("%+v", p)
	}
	p = ProblemOf(New(StoreWiped, "store wiped"))
	if p.Status != 409 || p.Code != StoreWiped {
		t.Fatalf("%+v", p)
	}
}
