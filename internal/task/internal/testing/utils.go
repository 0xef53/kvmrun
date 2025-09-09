package testing

import (
	"fmt"
	"strings"
)

func FormatResultString(want, got interface{}, name ...string) string {
	lines := make([]string, 0, 3)

	if len(name) == 0 {
		lines = append(lines, "got unexpected result:")
	} else {
		lines = append(lines, fmt.Sprintf("got unexpected result in '%s' test:", strings.Join(name, ", ")))
	}

	lines = append(lines, fmt.Sprintf("\twant:\t%v", want), fmt.Sprintf("\tgot:\t%v", got))

	return strings.Join(lines, "\n")
}
