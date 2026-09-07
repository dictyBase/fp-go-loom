package terminaleffect

import (
	E "github.com/IBM/fp-go/v2/either"
)

// foldBothArmsSend calls the terminal effect in both arms.
func foldBothArmsSend(res E.Either[error, message]) int {
	return E.Fold(
		func(err error) int { return sendDLQ(err) },
		func(msg message) int { return sendOK(msg) },
	)(res)
}

func sendOK(msg message) int { send(msg); return 1 }
func sendDLQ(err error) int  { send(message{Topic: "dlq"}); return 0 }
