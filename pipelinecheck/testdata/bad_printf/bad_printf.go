// Package bad_printf is a negative fixture for the safe-Printf rule.
package bad_printf

import (
	IO "github.com/IBM/fp-go/v2/io"
)

type State struct {
	APIKey string
}

func Bad(s State) IO.IO[State] {
	return IO.Printf[State]("current state")
}
