//go:build !codeanalysis

package generic

import (
	"github.com/brevdev/brev-cli/pkg/entity"
	orderedmap "github.com/wk8/go-ordered-map/v2"
)

func MakeVirtualProjectMap() *orderedmap.OrderedMap[string, map[string][]entity.Workspace] {
	vpMap := orderedmap.New[string, map[string][]entity.Workspace]()
	return vpMap
}
