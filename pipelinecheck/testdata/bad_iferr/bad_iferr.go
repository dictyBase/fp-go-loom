// Package bad_iferr is a negative fixture for the TryCatch-if-err rule.
package bad_iferr

import (
	"fmt"

	IOE "github.com/IBM/fp-go/v2/ioeither"
)

type Thing struct{}

func get() (*Thing, error) { return nil, fmt.Errorf("boom") }

func Bad() IOE.IOEither[error, *Thing] {
	return IOE.TryCatchError(func() (*Thing, error) {
		x, err := get()
		if err != nil {
			return nil, fmt.Errorf("get: %w", err)
		}
		return x, nil
	})
}
