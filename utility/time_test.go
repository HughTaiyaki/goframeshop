package utility

import (
	"testing"
	"time"

	"github.com/gogf/gf/v2/os/gtime"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestTimeConversions(t *testing.T) {
	if SafeConvertTime(nil) != nil || TimeFromTimestamp(nil) != nil {
		t.Fatal("nil must stay nil")
	}
	if SafeConvertTime(gtime.New(time.Time{})) != nil {
		t.Fatal("zero time must become nil, not 1970")
	}
	instant := time.Date(2026, 10, 20, 6, 59, 45, 0, time.UTC)
	got := TimeFromTimestamp(timestamppb.New(instant))
	if !got.Time.Equal(instant) {
		t.Fatalf("instant changed: %v", got)
	}
	// Rendered without a zone, so it must be rendered in local time.
	if want := instant.Local().Format("2006-01-02 15:04:05"); got.String() != want {
		t.Fatalf("rendered %q, want local %q", got.String(), want)
	}
}
