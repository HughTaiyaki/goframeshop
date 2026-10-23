// Package utility holds code shared by every service in this MonoRepo.
package utility

import (
	"github.com/gogf/gf/v2/os/gtime"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// SafeConvertTime converts a nullable database timestamp into a Protobuf
// timestamp. A GoFrame zero time or a nil pointer becomes nil instead of the
// Protobuf epoch (1970-01-01), so clients can tell "never set" from "set".
func SafeConvertTime(t *gtime.Time) *timestamppb.Timestamp {
	if t == nil || t.IsZero() {
		return nil
	}
	return timestamppb.New(t.Time)
}

// TimeFromTimestamp converts a Protobuf timestamp back into a GoFrame time.
// The gateway needs the reverse direction because it renders JSON, not Protobuf.
//
// The result is in the server's local zone. GoFrame renders times as
// "2006-01-02 15:04:05" with no zone, so a UTC value would silently read as
// eight hours off in Asia/Shanghai; local time matches what MySQL stores
// (the DSNs use loc=Local) and what people expect to read.
func TimeFromTimestamp(ts *timestamppb.Timestamp) *gtime.Time {
	if ts == nil {
		return nil
	}
	return gtime.New(ts.AsTime().Local())
}
