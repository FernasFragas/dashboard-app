package api

import (
	"fmt"
	"strconv"
	"strings"
)

func etag(id int64, version int) string {
	return fmt.Sprintf(`W/"%d-%d"`, id, version)
}

func parseIfMatch(header string, id int64) (int, error) {
	if strings.TrimSpace(header) == "" {
		return 0, preconditionError{status: 428, message: "If-Match header is required"}
	}

	prefix := fmt.Sprintf(`W/"%d-`, id)
	if !strings.HasPrefix(header, prefix) || !strings.HasSuffix(header, `"`) {
		return 0, preconditionError{status: 400, message: "invalid If-Match header"}
	}

	rawVersion := strings.TrimSuffix(strings.TrimPrefix(header, prefix), `"`)
	version, err := strconv.Atoi(rawVersion)
	if err != nil || version <= 0 {
		return 0, preconditionError{status: 400, message: "invalid If-Match header"}
	}

	return version, nil
}

type preconditionError struct {
	status  int
	message string
}

func (e preconditionError) Error() string { return e.message }
