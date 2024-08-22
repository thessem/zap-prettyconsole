package prettyconsole

import (
	"sync"

	"go.uber.org/zap/buffer"
)

var (
	_prettyConsolePool = sync.Pool{New: func() interface{} {
		return &prettyConsoleEncoder{}
	}}
	_recorderPool = sync.Pool{New: func() interface{} {
		return &recordingEncoder{}
	}}
	_bufferPool    = buffer.NewPool()
	_bufferPoolGet = _bufferPool.Get
)

func getRecordingEncoder() *recordingEncoder {
	return _recorderPool.Get().(*recordingEncoder)
}

func getPrettyConsoleEncoder() *prettyConsoleEncoder {
	return _prettyConsolePool.Get().(*prettyConsoleEncoder)
}

func putPrettyConsoleEncoder(e *prettyConsoleEncoder) {
	e.cfg = nil
	if e.buf != nil {
		putBuffer(e.buf)
	}
	e.buf = nil

	e.namespaceIndent = 0
	e.inList = false
	e.listSep = ""
	e._listSepSpace = ""
	e._listSepComma = ""

	_prettyConsolePool.Put(e)
}

func getBuffer() *buffer.Buffer {
	return _bufferPool.Get()
}

func putBuffer(b *buffer.Buffer) {
	b.Free()
}
