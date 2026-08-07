package filejoinfixture

import (
	arr "github.com/IBM/fp-go/v2/array"
	"path/filepath"
)

func bad(root string) []string {
	return arr.Map(func(match string) string {
		return filepath.Join(root, filepath.FromSlash(match))
	})([]string{"x"})
}
