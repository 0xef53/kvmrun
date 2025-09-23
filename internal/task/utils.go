package task

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// GetShortID returns the short form (first segment) of a UUID string representing a task ID.
// Returns an error if the input string is not a valid UUID.
func GetShortID(tid string) (string, error) {
	uuid, err := uuid.Parse(tid)
	if err != nil {
		return "", fmt.Errorf("broken UUID: %w", err)
	}

	return strings.Split(uuid.String(), "-")[0], nil
}
