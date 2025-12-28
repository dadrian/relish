package internal

import (
	"reflect"
	"strconv"
	"strings"
)

// ParseRelishTag parses `relish:"<id>[,optional][,omitempty]"` into components.
// Returns (id, optional, omitempty, ok).
func ParseRelishTag(f reflect.StructField) (int, bool, bool, bool) {
	tag := f.Tag.Get("relish")
	if tag == "" || tag == "-" {
		return 0, false, false, false
	}
	parts := strings.Split(tag, ",")
	id64, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || id64 < 0 || id64 >= 0x80 {
		return 0, false, false, false
	}
	var optional, omitempty bool
	for _, p := range parts[1:] {
		switch strings.TrimSpace(p) {
		case "optional":
			optional = true
		case "omitempty":
			omitempty = true
		}
	}
	return int(id64), optional, omitempty, true
}
