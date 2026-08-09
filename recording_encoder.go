package prettyconsole

import (
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/buffer"
	"go.uber.org/zap/zapcore"
)

// recordingEncoder is for recording fields added to the encoder with `With`. We
// need to make sure we record fields separately since we need to sort
// them later on with the main output log function
type recordingEncoder struct {
	fields []zapcore.Field
	e      prettyConsoleEncoder
	// prep caches sort and render work for the recorded fields. It is
	// built lazily on first EncodeEntry: by then the field set is final,
	// because zap always clones the encoder before adding fields.
	prep atomic.Pointer[preparedContext]
}

// preparedContext is immutable once published.
type preparedContext struct {
	// sorted holds the recorded fields with every namespace segment
	// pre-sorted, so per-entry sorting of the merged field list is O(n)
	// for the already-ordered prefix.
	sorted []zapcore.Field
	// rendered caches the fully rendered field bytes per colour level,
	// used when an entry adds no fields of its own. Like zap's built-in
	// encoders, this means context fields are rendered once, not per
	// line: marshalers and errors in accumulated context are frozen at
	// first use.
	rendered [len(defaultColours)]atomic.Pointer[[]byte]
}

// Clone implements zapcore.Encoder
func (r *recordingEncoder) Clone() zapcore.Encoder {
	clone := getRecordingEncoder()
	clone.e = r.e // This will not have been modified
	clone.fields = make([]zapcore.Field, len(r.fields))
	copy(clone.fields, r.fields)
	// The clone is about to have fields appended: it must not inherit
	// this encoder's prepared cache.
	clone.prep.Store(nil)
	return clone
}

func (r *recordingEncoder) prepared() *preparedContext {
	if p := r.prep.Load(); p != nil {
		return p
	}
	sorted := make([]zapcore.Field, len(r.fields))
	copy(sorted, r.fields)
	sortFieldSegments(sorted)
	p := &preparedContext{sorted: sorted}
	if r.prep.CompareAndSwap(nil, p) {
		return p
	}
	return r.prep.Load()
}

// renderedContext returns the cached rendering of the recorded fields for
// a level, rendering and publishing it on first use.
func (r *recordingEncoder) renderedContext(p *preparedContext, lvl zapcore.Level) []byte {
	idx := colourIdx(lvl)
	if b := p.rendered[idx].Load(); b != nil {
		return *b
	}
	enc := r.e
	enc.buf = getBuffer()
	enc.level = lvl
	// This is the encoder state encodePreamble leaves behind.
	enc.inList = true
	enc.encodeFields(p.sorted)
	b := append([]byte(nil), enc.buf.Bytes()...)
	putBuffer(enc.buf)
	p.rendered[idx].CompareAndSwap(nil, &b)
	return *p.rendered[idx].Load()
}

// EncodeEntry implements zapcore.Encoder
func (r *recordingEncoder) EncodeEntry(entry zapcore.Entry, fields []zapcore.Field) (*buffer.Buffer, error) {
	if len(r.fields) == 0 {
		// No accumulated context: encode the caller's fields directly.
		// They are still copied because EncodeEntry sorts in place.
		return r.encodeMerged(entry, nil, fields)
	}
	p := r.prepared()
	if len(fields) == 0 {
		// Fast path: nothing to interleave with the context, so reuse
		// its cached rendering for this level. The pooled encoder keeps
		// the copy from escaping through the preamble interfaces.
		enc := getPrettyConsoleEncoder()
		*enc = r.e
		enc.buf = getBuffer()
		enc.level = entry.Level
		enc.encodePreamble(entry)
		_, _ = enc.buf.Write(r.renderedContext(p, entry.Level))
		enc.encodeFinish(entry)
		buf := enc.buf
		enc.buf = nil
		putPrettyConsoleEncoder(enc)
		return buf, nil
	}
	return r.encodeMerged(entry, p.sorted, fields)
}

// encodeMerged renders context and entry fields together through a pooled
// scratch slice: EncodeEntry sorts in place, and neither the recorded
// fields nor the caller's slice may be mutated. The scratch is cleared
// before being returned so pooled entries do not pin field values.
func (r *recordingEncoder) encodeMerged(entry zapcore.Entry, context, fields []zapcore.Field) (*buffer.Buffer, error) {
	n := len(context) + len(fields)
	fp := _fieldsPool.Get().(*[]zapcore.Field)
	if cap(*fp) < n {
		*fp = make([]zapcore.Field, n)
	}
	fieldsClone := (*fp)[:n]
	copy(fieldsClone, context)
	copy(fieldsClone[len(context):], fields)
	buf, err := r.e.EncodeEntry(entry, fieldsClone)
	for i := range fieldsClone {
		fieldsClone[i] = zapcore.Field{}
	}
	_fieldsPool.Put(fp)
	return buf, err
}

var _fieldsPool = sync.Pool{New: func() interface{} {
	s := make([]zapcore.Field, 0, 16)
	return &s
}}

// AddArray implements zapcore.ObjectEncoder
func (r *recordingEncoder) AddArray(key string, marshaler zapcore.ArrayMarshaler) error {
	r.fields = append(r.fields, zap.Array(key, marshaler))
	return nil
}

// AddObject implements zapcore.ObjectEncoder
func (r *recordingEncoder) AddObject(key string, marshaler zapcore.ObjectMarshaler) error {
	r.fields = append(r.fields, zap.Object(key, marshaler))
	return nil
}

// AddBinary implements zapcore.ObjectEncoder
func (r *recordingEncoder) AddBinary(key string, value []byte) {
	r.fields = append(r.fields, zap.Binary(key, value))
}

// AddByteString implements zapcore.ObjectEncoder
func (r *recordingEncoder) AddByteString(key string, value []byte) {
	r.fields = append(r.fields, zap.ByteString(key, value))
}

// AddBool implements zapcore.ObjectEncoder
func (r *recordingEncoder) AddBool(key string, value bool) {
	r.fields = append(r.fields, zap.Bool(key, value))
}

// AddComplex128 implements zapcore.ObjectEncoder
func (r *recordingEncoder) AddComplex128(key string, value complex128) {
	r.fields = append(r.fields, zap.Complex128(key, value))
}

// AddComplex64 implements zapcore.ObjectEncoder
func (r *recordingEncoder) AddComplex64(key string, value complex64) {
	r.fields = append(r.fields, zap.Complex64(key, value))
}

// AddDuration implements zapcore.ObjectEncoder
func (r *recordingEncoder) AddDuration(key string, value time.Duration) {
	r.fields = append(r.fields, zap.Duration(key, value))
}

// AddFloat64 implements zapcore.ObjectEncoder
func (r *recordingEncoder) AddFloat64(key string, value float64) {
	r.fields = append(r.fields, zap.Float64(key, value))
}

// AddFloat32 implements zapcore.ObjectEncoder
func (r *recordingEncoder) AddFloat32(key string, value float32) {
	r.fields = append(r.fields, zap.Float32(key, value))
}

// AddInt implements zapcore.ObjectEncoder
func (r *recordingEncoder) AddInt(key string, value int) {
	r.fields = append(r.fields, zap.Int(key, value))
}

// AddInt64 implements zapcore.ObjectEncoder
func (r *recordingEncoder) AddInt64(key string, value int64) {
	r.fields = append(r.fields, zap.Int64(key, value))
}

// AddInt32 implements zapcore.ObjectEncoder
func (r *recordingEncoder) AddInt32(key string, value int32) {
	r.fields = append(r.fields, zap.Int32(key, value))
}

// AddInt16 implements zapcore.ObjectEncoder
func (r *recordingEncoder) AddInt16(key string, value int16) {
	r.fields = append(r.fields, zap.Int16(key, value))
}

// AddInt8 implements zapcore.ObjectEncoder
func (r *recordingEncoder) AddInt8(key string, value int8) {
	r.fields = append(r.fields, zap.Int8(key, value))
}

// AddString implements zapcore.ObjectEncoder
func (r *recordingEncoder) AddString(key, value string) {
	r.fields = append(r.fields, zap.String(key, value))
}

// AddTime implements zapcore.ObjectEncoder
func (r *recordingEncoder) AddTime(key string, value time.Time) {
	r.fields = append(r.fields, zap.Time(key, value))
}

// AddUint implements zapcore.ObjectEncoder
func (r *recordingEncoder) AddUint(key string, value uint) {
	r.fields = append(r.fields, zap.Uint(key, value))
}

// AddUint64 implements zapcore.ObjectEncoder
func (r *recordingEncoder) AddUint64(key string, value uint64) {
	r.fields = append(r.fields, zap.Uint64(key, value))
}

// AddUint32 implements zapcore.ObjectEncoder
func (r *recordingEncoder) AddUint32(key string, value uint32) {
	r.fields = append(r.fields, zap.Uint32(key, value))
}

// AddUint16 implements zapcore.ObjectEncoder
func (r *recordingEncoder) AddUint16(key string, value uint16) {
	r.fields = append(r.fields, zap.Uint16(key, value))
}

// AddUint8 implements zapcore.ObjectEncoder
func (r *recordingEncoder) AddUint8(key string, value uint8) {
	r.fields = append(r.fields, zap.Uint8(key, value))
}

// AddUintptr implements zapcore.ObjectEncoder
func (r *recordingEncoder) AddUintptr(key string, value uintptr) {
	r.fields = append(r.fields, zap.Uintptr(key, value))
}

// AddReflected implements zapcore.ObjectEncoder
func (r *recordingEncoder) AddReflected(key string, value interface{}) error {
	r.fields = append(r.fields, zap.Reflect(key, value))
	return nil
}

// OpenNamespace implements zapcore.ObjectEncoder
func (r *recordingEncoder) OpenNamespace(key string) {
	r.fields = append(r.fields, zap.Namespace(key))
}
