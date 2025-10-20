package checkstest

import (
	"fmt"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

func Passf(name, format string, args ...any) checks.CheckResult {
	return checks.CheckResult{
		Name:    name,
		Status:  checks.Pass,
		Message: sprintf(format, args...),
	}
}

func Warnf(name, format string, args ...any) checks.CheckResult {
	return checks.CheckResult{
		Name:    name,
		Status:  checks.Warn,
		Message: sprintf(format, args...),
	}
}

func Failf(name, format string, args ...any) checks.CheckResult {
	return checks.CheckResult{
		Name:    name,
		Status:  checks.Fail,
		Message: sprintf(format, args...),
	}
}

func Errorf(name, format string, args ...any) checks.CheckResult {
	return checks.CheckResult{
		Name:    name,
		Status:  checks.Error,
		Message: sprintf(format, args...),
	}
}

func sprintf(format string, args ...any) string {
	if len(args) == 0 {
		return format
	}
	return fmt.Sprintf(format, args...)
}
