package prefixid

import (
	"fmt"

	"github.com/segmentio/ksuid"
)

type PrefixID string

const prefixSep = "-"

// New generates a unique ID that can be used as an identifier for an entity.
func New(prefix string) PrefixID {
	return PrefixID(fmt.Sprintf("%s%s%s", prefix, prefixSep, ksuid.New().String()))
}
