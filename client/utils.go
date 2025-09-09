package client

import (
	"strings"
)

func countRequiredArgs(s string) (c int) {
	for _, v := range strings.Fields(s) {
		if !strings.HasPrefix(v, "[") {
			c++
		}
	}
	return c
}
