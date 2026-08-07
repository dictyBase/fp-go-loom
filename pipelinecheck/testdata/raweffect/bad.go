package raweffectfixture

import (
	f "fmt"
	IOE "github.com/IBM/fp-go/v2/ioeither"
)

func fetch() (string, error) { return "", nil }

func bad() IOE.IOEither[error, string] {
	return IOE.TryCatchError(func() (string, error) {
		value, err := fetch()
		if err != nil {
			return "", f.Errorf("fetch: %w", err)
		}
		return value, nil
	})
}
