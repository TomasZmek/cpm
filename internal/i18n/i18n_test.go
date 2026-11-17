package i18n

import (
	"regexp"
	"strings"
	"testing"
)

func TestSelectableLanguagesAreComplete(t *testing.T) {
	if err := Init(); err != nil {
		t.Fatal(err)
	}
	selectable := map[string]bool{}
	for _, l := range SelectableLanguages() {
		selectable[l.Code] = true
		if missing := MissingKeys(l.Code); len(missing) > 0 {
			t.Logf("%s is %.0f%% translated, missing %d keys: %s", l.Code, l.Coverage*100, len(missing), strings.Join(missing, ", "))
		}
	}
	for _, code := range []string{"en", "cs", "ko"} {
		if !selectable[code] {
			t.Errorf("%s should be selectable (coverage %.0f%%)", code, Coverage(code)*100)
		}
	}
	for code := range AvailableLanguages {
		if !selectable[code] && IsValidLanguage(code) {
			t.Errorf("%s is not selectable but accepted as valid", code)
		}
		if !selectable[code] {
			t.Logf("%s hidden: %.0f%% translated (needs %.0f%%)", code, Coverage(code)*100, MinCoverage*100)
		}
	}
}

// Plural-Forms headers must match the hard-coded pluralRules.
func TestPluralHeadersMatchRules(t *testing.T) {
	re := regexp.MustCompile(`nplurals=(\d+)`)
	for code := range AvailableLanguages {
		data, err := localesFS.ReadFile("locales/" + code + "/LC_MESSAGES/messages.po")
		if err != nil {
			t.Fatal(err)
		}
		m := re.FindSubmatch(data)
		if m == nil {
			t.Errorf("%s: no Plural-Forms header", code)
			continue
		}
		rule := pluralRules[code]
		max := 0
		for n := 0; n < 200; n++ {
			if f := rule(n); f > max {
				max = f
			}
		}
		if want := string(m[1]); want != string(rune('0'+max+1)) {
			t.Errorf("%s: header says nplurals=%s but rule uses %d forms", code, want, max+1)
		}
	}
}

func TestPluralForms(t *testing.T) {
	if err := Init(); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		lang string
		n    int
		want string
	}{
		{"en", 1, "1 rule"},
		{"en", 0, "0 rules"},
		{"en", 5, "5 rules"},
		{"cs", 1, "1 pravidlo"},
		{"cs", 3, "3 pravidla"},
		{"cs", 5, "5 pravidel"},
		{"cs", 0, "0 pravidel"},
		{"ko", 1, "규칙 1개"},
		{"ko", 7, "규칙 7개"},
		{"de", 2, "2 rules"}, // untranslated language falls back to English
	}
	for _, tc := range cases {
		if got := TN(tc.lang, "count_rules", "count_rules_plural", tc.n, tc.n); got != tc.want {
			t.Errorf("TN(%s, %d) = %q, want %q", tc.lang, tc.n, got, tc.want)
		}
	}
}

// Every plural key in English must exist in cs and ko with all forms of the
// language's plural rule.
func TestPluralKeysComplete(t *testing.T) {
	if err := Init(); err != nil {
		t.Fatal(err)
	}
	en := locales["en"]
	if len(en.plural) == 0 {
		t.Fatal("no plural keys in en")
	}
	for _, code := range []string{"en", "cs", "ko"} {
		ld := locales[code]
		nforms := 0
		for n := 0; n < 200; n++ {
			if f := ld.selectN(n) + 1; f > nforms {
				nforms = f
			}
		}
		for key := range en.plural {
			forms, ok := ld.plural[key]
			if !ok {
				t.Errorf("%s: plural key %q missing", code, key)
				continue
			}
			for f := 0; f < nforms; f++ {
				if forms[f] == "" {
					t.Errorf("%s: plural key %q has no form %d", code, key, f)
				}
			}
		}
	}
}
