package i18n

import (
	"embed"
	"fmt"
	"strings"
	"sync"

	"github.com/leonelquinteros/gotext"
)

//go:embed locales
var localesFS embed.FS

// AvailableLanguages maps language codes to display names.
var AvailableLanguages = map[string]string{
	"en": "English",
	"cs": "Čeština",
	"ko": "한국어",
}

// pluralFunc selects the plural form index for a given n.
type pluralFunc func(n int) int

// pluralRules contains hardcoded plural selectors for supported languages.
// These match the Plural-Forms headers in the corresponding .po files.
var pluralRules = map[string]pluralFunc{
	"en": func(n int) int {
		if n != 1 {
			return 1
		}
		return 0
	},
	"cs": func(n int) int {
		if n == 1 {
			return 0
		}
		if n >= 2 && n <= 4 {
			return 1
		}
		return 2
	},
	"ko": func(_ int) int { return 0 },
}

// langData holds extracted translation maps for one language.
type langData struct {
	singular map[string]string          // msgid → msgstr
	plural   map[string]map[int]string  // msgid → {form_index → msgstr}
	selectN  pluralFunc
}

var (
	mu      sync.RWMutex
	locales = map[string]*langData{}
)

// Init loads all PO files from the embedded locales directory.
func Init() error {
	for lang := range AvailableLanguages {
		path := "locales/" + lang + "/LC_MESSAGES/messages.po"
		data, err := localesFS.ReadFile(path)
		if err != nil {
			return fmt.Errorf("loading locale %q: %w", lang, err)
		}

		po := gotext.NewPo()
		po.Parse(data)

		ld := &langData{
			singular: make(map[string]string),
			plural:   make(map[string]map[int]string),
			selectN:  pluralRules[lang],
		}
		if ld.selectN == nil {
			ld.selectN = pluralRules["en"]
		}

		for msgid, tr := range po.GetDomain().GetTranslations() {
			// Singular: stored in Trs[0]
			if s, ok := tr.Trs[0]; ok && s != "" {
				ld.singular[msgid] = s
			}
			// Plural: entries with a PluralID have form[n] in Trs
			if tr.PluralID != "" && len(tr.Trs) > 0 {
				forms := make(map[int]string, len(tr.Trs))
				for form, s := range tr.Trs {
					forms[form] = s
				}
				ld.plural[msgid] = forms
			}
		}

		mu.Lock()
		locales[lang] = ld
		mu.Unlock()
	}
	return nil
}

// T returns the translation for key in the given language.
// Falls back to English when key is missing in the requested language.
// Supports {0}, {1}, … positional placeholders in translation strings.
func T(lang, key string, args ...interface{}) string {
	mu.RLock()
	ld := locales[lang]
	enLd := locales["en"]
	mu.RUnlock()

	result := key
	if ld != nil {
		if s, ok := ld.singular[key]; ok {
			result = s
		}
	}

	// Fall back to English when key was not translated.
	if result == key && lang != "en" && enLd != nil {
		if s, ok := enLd.singular[key]; ok {
			result = s
		}
	}

	return applyArgs(result, args)
}

// TN returns the plural-aware translation for the given singular/plural keys.
// n determines which plural form to select according to the language's Plural-Forms rule.
// Supports {0}, {1}, … positional placeholders.
func TN(lang, singular, plural string, n int, args ...interface{}) string {
	mu.RLock()
	ld := locales[lang]
	enLd := locales["en"]
	mu.RUnlock()

	result := singular
	if n != 1 {
		result = plural
	}

	if ld != nil {
		if forms, ok := ld.plural[singular]; ok {
			form := ld.selectN(n)
			if s, ok := forms[form]; ok && s != "" {
				result = s
			}
		}
	}

	// Fall back to English.
	if (result == singular || result == plural) && lang != "en" && enLd != nil {
		if forms, ok := enLd.plural[singular]; ok {
			form := enLd.selectN(n)
			if s, ok := forms[form]; ok && s != "" {
				result = s
			}
		}
	}

	return applyArgs(result, args)
}

// applyArgs replaces {0}, {1}, … placeholders with the corresponding args.
func applyArgs(s string, args []interface{}) string {
	for i, arg := range args {
		s = strings.ReplaceAll(s, fmt.Sprintf("{%d}", i), fmt.Sprint(arg))
	}
	return s
}

// IsValidLanguage reports whether lang is a supported language code.
func IsValidLanguage(lang string) bool {
	_, ok := AvailableLanguages[lang]
	return ok
}

// GetLanguages returns the map of supported language codes to display names.
func GetLanguages() map[string]string {
	return AvailableLanguages
}
