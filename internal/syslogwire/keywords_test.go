package syslogwire

import "testing"

func TestFacilityKeywords(t *testing.T) {
	want := []string{
		"kern", "user", "mail", "daemon", "auth", "syslog", "lpr", "news",
		"uucp", "cron", "authpriv", "ftp", "ntp", "audit", "console", "cron2",
		"local0", "local1", "local2", "local3", "local4", "local5", "local6", "local7",
	}
	if len(want) != 24 {
		t.Fatal("docs/02 lists facilities 0–23")
	}
	for i, k := range want {
		if FacilityKeyword(uint8(i)) != k {
			t.Errorf("facility %d = %q, want %q", i, FacilityKeyword(uint8(i)), k)
		}
		n, ok := LookupFacility(k)
		if !ok || n != uint8(i) {
			t.Errorf("LookupFacility(%q) = %d %v", k, n, ok)
		}
	}
	if FacilityKeyword(24) != "" {
		t.Fatal("facility 24 should have no keyword")
	}
	if n, ok := LookupFacility("20"); !ok || n != 20 {
		t.Fatalf("numeric facility 20 = %d %v", n, ok)
	}
	if _, ok := LookupFacility("24"); ok {
		t.Fatal("facility 24 must not lookup")
	}
}

func TestSeverityKeywords(t *testing.T) {
	want := []string{"emerg", "alert", "crit", "err", "warning", "notice", "info", "debug"}
	for i, k := range want {
		if SeverityKeyword(uint8(i)) != k {
			t.Errorf("severity %d = %q, want %q", i, SeverityKeyword(uint8(i)), k)
		}
		n, ok := LookupSeverity(k)
		if !ok || n != uint8(i) {
			t.Errorf("LookupSeverity(%q) = %d %v", k, n, ok)
		}
	}
	if _, ok := LookupSeverity("8"); ok {
		t.Fatal("severity 8 must not lookup")
	}
}

func TestPRIComposition(t *testing.T) {
	cases := []struct {
		pri      uint8
		facility uint8
		severity uint8
		facKw    string
		sevKw    string
	}{
		{0, 0, 0, "kern", "emerg"},
		{13, 1, 5, "user", "notice"},
		{14, 1, 6, "user", "info"},
		{30, 3, 6, "daemon", "info"},
		{34, 4, 2, "auth", "crit"},
		{165, 20, 5, "local4", "notice"},
		{191, 23, 7, "local7", "debug"},
	}
	for _, tc := range cases {
		if tc.pri != tc.facility*8+tc.severity {
			t.Errorf("PRI %d != %d*8+%d", tc.pri, tc.facility, tc.severity)
		}
		if FacilityKeyword(tc.facility) != tc.facKw {
			t.Errorf("PRI %d facility keyword %q", tc.pri, FacilityKeyword(tc.facility))
		}
		if SeverityKeyword(tc.severity) != tc.sevKw {
			t.Errorf("PRI %d severity keyword %q", tc.pri, SeverityKeyword(tc.severity))
		}
	}
}
