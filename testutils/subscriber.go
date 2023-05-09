package testutils

import (
	"time"

	"github.com/google/uuid"
)

const TestEmail = "foo@bar.com"
const TestTimeStr = "Fri, 18 Sep 1970 12:45:00 +0000"

var TestUid uuid.UUID = uuid.MustParse("00000000-1111-2222-3333-444444444444")

var TestTimestamp time.Time

func init() {
	var err error
	TestTimestamp, err = time.Parse(time.RFC1123Z, TestTimeStr)

	if err != nil {
		panic("failed to parse testTimestamp: " + err.Error())
	}
}