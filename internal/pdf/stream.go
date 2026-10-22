package pdf

import (
	"bytes"
	"context"
	"fmt"
	"github.com/chromedp/cdproto/cdp"
	"io"

	cdprotoio "github.com/chromedp/cdproto/io"
)

// Credits: https://raw.githubusercontent.com/mafredri/cdp/3c5eab7ffc5cbee667b0a813ce470ac423792811/protocol/io/stream_reader.go.
type streamReader struct {
	ctx     context.Context
	session cdp.Session
	handle  cdprotoio.StreamHandle
	r       io.Reader
	pos     int
	eof     bool
}

// Read a chunk of the stream.
func (reader *streamReader) Read(p []byte) (n int, err error) {
	if reader.r != nil {
		// Continue reading from buffer.
		return reader.read(p)
	}

	if reader.eof {
		return 0, io.EOF
	}

	if len(p) == 0 {
		return 0, nil
	}

	// Chromium might have an off-by-one when deciding the maximum size (at
	// least for base64 encoded data), usually it will overflow. We subtract
	// one to make sure it fits into p.
	size := len(p) - 1
	if size < 1 {
		// Safety-check to avoid crashing Chrome (e.g. via SetSize(-1)).
		size = 1
	}

	reply, err := reader.next(reader.pos, size)
	if err != nil {
		return 0, err
	}

	reader.eof = reply.EOF

	// cdproto decodes base64 data, so reply.Data holds the raw bytes.
	reader.r = bytes.NewReader(reply.Data)

	return reader.read(p)
}

// Close closes the stream, discard any temporary backing storage.
func (reader *streamReader) Close() error {
	_, err := cdp.Call(reader.ctx, reader.session, cdprotoio.Close, cdprotoio.CloseParams{Handle: reader.handle})
	if err == nil {
		return nil
	}

	return fmt.Errorf("close Chromium stream: %w", err)
}

func (reader *streamReader) next(pos, size int) (cdprotoio.ReadResult, error) {
	offset := int64(pos)
	res, err := cdp.Call(reader.ctx, reader.session, cdprotoio.Read, cdprotoio.ReadParams{
		Handle: reader.handle,
		Offset: &offset,
		Size:   int64(size),
	})

	if err == nil {
		return res, nil
	}

	return res, fmt.Errorf("execute IO.read command: %w", err)
}

func (reader *streamReader) read(p []byte) (n int, err error) {
	n, err = reader.r.Read(p)
	reader.pos += n

	if !reader.eof && err == io.EOF {
		reader.r = nil
		err = nil
	}

	return n, err
}

// Interface guards.
var (
	_ io.Reader = (*streamReader)(nil)
)
