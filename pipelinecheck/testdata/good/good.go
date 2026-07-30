// Package good is a positive fixture: it follows both rules.
package good

import (
	IO "github.com/IBM/fp-go/v2/io"
	IOE "github.com/IBM/fp-go/v2/ioeither"
)

func GoodIO(path string) IO.IO[string] {
	return IO.Printf[string]("path=%s")
}

func GoodEffect(s string) IOE.IOEither[error, string] {
	return IOE.TryCatchError(func() (string, error) {
		return s, nil
	})
}
