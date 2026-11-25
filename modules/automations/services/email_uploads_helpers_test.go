package services_test

import (
	"bytes"
	"io"
)

func bytesReader(s string) io.ReadSeeker { return bytes.NewReader([]byte(s)) }
