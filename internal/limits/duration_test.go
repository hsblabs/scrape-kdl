package limits

import (
	"testing"
	"time"
)

func TestMillisecondsBounds(t *testing.T) {
	value, ok := Milliseconds(int(MaxMilliseconds))
	if !ok || value != time.Duration(MaxMilliseconds)*time.Millisecond {
		t.Fatalf("maximum duration = %v, %v", value, ok)
	}
	for _, invalid := range []int{0, -1, int(MaxMilliseconds) + 1} {
		if value, ok := Milliseconds(invalid); ok || value != 0 {
			t.Fatalf("Milliseconds(%d) = %v, %v", invalid, value, ok)
		}
	}
}
