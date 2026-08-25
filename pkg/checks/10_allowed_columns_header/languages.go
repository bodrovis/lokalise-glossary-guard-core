package allowed_columns_header

import "strings"

type declaredLanguages struct {
	order  []string
	set    map[string]struct{}
	labels map[string]string
}

func newDeclaredLanguages(langs []string) declaredLanguages {
	out := declaredLanguages{
		set:    make(map[string]struct{}, len(langs)),
		labels: make(map[string]string, len(langs)),
	}

	for _, lang := range langs {
		label := strings.TrimSpace(lang)
		key := normalizeLangKey(label)

		if key == "" {
			continue
		}

		if _, exists := out.set[key]; exists {
			continue
		}

		out.set[key] = struct{}{}
		out.labels[key] = label
		out.order = append(out.order, key)
	}

	return out
}

func (d declaredLanguages) hasAny() bool {
	return len(d.order) > 0
}

func (d declaredLanguages) contains(lang string) bool {
	_, ok := d.set[normalizeLangKey(lang)]
	return ok
}
