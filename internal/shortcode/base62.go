// Package shortcode encodes/decodes the numeric ID used internally to the
// user-facing short path. Base62 is chosen for a compact URL-safe alphabet
// (0-9A-Za-z) with no reserved characters.
package shortcode

import (
	base62 "github.com/alextanhongpin/base62"
)

// ShortPathFromID encodes a sequential ID from the Redis global counter into
// a base62 short path. IDs are monotonically increasing so the path grows
// gradually: 1 char for the first 62 URLs, 2 for the next 3844, etc.
func ShortPathFromID(id int64) string {
	return base62.Encode(uint64(id))
}
