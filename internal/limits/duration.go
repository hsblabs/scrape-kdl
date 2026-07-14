package limits

import "time"

// MaxMilliseconds is the largest whole-millisecond value representable by
// time.Duration without overflow.
const MaxMilliseconds int64 = int64(^uint64(0)>>1) / int64(time.Millisecond)

func Milliseconds(value int) (time.Duration, bool) {
	if value < 1 || int64(value) > MaxMilliseconds {
		return 0, false
	}
	return time.Duration(value) * time.Millisecond, true
}
