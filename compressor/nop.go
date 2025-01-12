package compressor

import (
	"io"
)

type nopWriteCloser struct {
	io.Writer
}

func (nopWriteCloser) Close() error {
	return nil
}

func (nopWriteCloser) Reset(io.Writer) {}
