package util

import "log"

var Log = log.Default()

func Assert(condition bool, msg string) {
	if !condition {
		panic(msg)
	}
}
