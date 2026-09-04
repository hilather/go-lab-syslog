package main

import "testing"

func TestCheckChangelogRequiresEntry(t *testing.T) {
	err := checkChangelog([]string{"internal/syslogserver/server.go"})
	if err == nil {
		t.Fatal("expected missing changelog")
	}
	err = checkChangelog([]string{"internal/syslogserver/server.go", "CHANGELOG.md"})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCheckChangelogIgnoresTestsAndUnobservable(t *testing.T) {
	if err := checkChangelog([]string{"internal/store/store_test.go", "docs/01-architecture.md"}); err != nil {
		t.Fatal(err)
	}
}

func TestObservableRel(t *testing.T) {
	if !observableRel("docs/02-syslog-semantics.md") {
		t.Fatal("syslog-semantics should be observable")
	}
	if !observableRel("cmd/labsyslog/main.go") {
		t.Fatal("cmd should be observable")
	}
	if observableRel("internal/store/store_test.go") {
		t.Fatal("tests are not observable")
	}
	if observableRel("CHANGELOG.md") {
		t.Fatal("changelog itself is not an observable trigger")
	}
}
