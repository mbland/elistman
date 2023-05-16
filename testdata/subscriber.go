package testdata

import (
	"time"

	"github.com/google/uuid"
)

const TestEmail = "foo@bar.com"
const TestTimeStr = "Fri, 18 Sep 1970 12:45:00 +0000"
const TestUidStr = "00000000-1111-2222-3333-444444444444"

var TestUid uuid.UUID = uuid.MustParse(TestUidStr)

var TestTimestamp time.Time = func() time.Time {
	var ts time.Time
	var err error

	if ts, err = time.Parse(time.RFC1123Z, TestTimeStr); err != nil {
		panic("failed to parse TestTimeStr: " + err.Error())
	}
	return ts
}()
