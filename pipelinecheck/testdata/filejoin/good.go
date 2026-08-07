package filejoinfixture

import (
	A "github.com/IBM/fp-go/v2/array"
	F "github.com/IBM/fp-go/v2/function"
	FILE "github.com/IBM/fp-go/v2/file"
)

func good(root string) []string {
	mapper := F.Pipe1(root, F.Flip(FILE.Join))
	return A.Map(mapper)([]string{"x"})
}
