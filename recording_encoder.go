package prettyconsole

import (
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
}

// Clone implements zapcore.Encoder
func (r *recordingEncoder) Clone() zapcore.Encoder {
	clone := getRecordingEncoder()
	clone.e = r.e // This will not have been modified
	clone.fields = make([]zapcore.Field, len(r.fields))
	copy(clone.fields, r.fields)
	return clone
}

// EncodeEntry implements zapcore.Encoder
func (r recordingEncoder) EncodeEntry(entry zapcore.Entry, fields []zapcore.Field) (*buffer.Buffer, error) {
	// Must copy r's fields because encode entry will sort them. Copying fields
	// too because it might be surprißsing to have the slice change under the
	// caller!.
	fieldsClone := make([]zapcore.Field, len(r.fields)+len(fields))
	copy(fieldsClone, r.fields)
	copy(fieldsClone[len(r.fields):], fields)
	return r.e.EncodeEntry(entry, fieldsClone)
}

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
	r.fields = append(r.fields, zap.Binary(key, value))
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
