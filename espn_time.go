package sports

import (
	"strings"
	"time"
)

// ESPNTime is a wrapper around time.Time that can unmarshal
// both full RFC3339 timestamps and the shorter “YYYY-MM-DDThh:mmZ”
// strings returned by some ESPN endpoints.
type ESPNTime struct {
	time.Time
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (t *ESPNTime) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		return nil
	}

	var parseErr error
	layouts := []string{
		time.RFC3339,           // 2006-01-02T15:04:05Z07:00
		"2006-01-02T15:04Z07:00", // 2006-01-02T15:04Z (no seconds)
	}

	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, s); err == nil {
			t.Time = parsed
			return nil
		} else {
			parseErr = err
		}
	}
	return parseErr
}
