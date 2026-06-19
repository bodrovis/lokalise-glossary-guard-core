package semicolon_separator

import (
	"context"
)

type separatorReport struct {
	semicolonOK bool
	commaOK     bool
	tabOK       bool
}

func detectSeparators(ctx context.Context, data []byte) (separatorReport, error) {
	semicolonOK, err := attemptRectParse(ctx, data, ';')
	if err != nil {
		return separatorReport{}, err
	}
	if semicolonOK {
		return separatorReport{semicolonOK: true}, nil
	}

	commaOK, err := attemptRectParse(ctx, data, ',')
	if err != nil {
		return separatorReport{}, err
	}

	tabOK, err := attemptRectParse(ctx, data, '\t')
	if err != nil {
		return separatorReport{}, err
	}

	return separatorReport{
		commaOK: commaOK,
		tabOK:   tabOK,
	}, nil
}

func separatorFailureMessage(report separatorReport) string {
	switch {
	case report.commaOK && !report.tabOK:
		return "file appears to use commas as separators; expected semicolons (;)"
	case report.tabOK && !report.commaOK:
		return "file appears to use tabs as separators; expected semicolons (;)"
	default:
		return "could not confirm consistent semicolon-separated format; cannot confidently detect an alternative delimiter"
	}
}
