package email

import (
	"fmt"
	"testing"

	"github.com/google/uuid"
	"gotest.tools/assert"
)

const testUnsubEmail = "unsubscribe@foo.com"
const testUnsubBaseUrl = "https://foo.com/email/unsubscribe/"
const testUid = "00000000-1111-2222-3333-444444444444"

func TestSubscriber(t *testing.T) {
	setup := func() *Subscriber {
		sub := &Subscriber{
			Email: "subscriber@foo.com",
			Uid:   uuid.MustParse(testUid),
		}
		sub.SetUnsubscribeInfo(testUnsubEmail, testUnsubBaseUrl)
		return sub
	}

	t.Run("SetUnsubscribeInfoSetsPrivateUnsubFields", func(t *testing.T) {
		sub := setup()

		const mailtoFmt = "mailto:%s?subject=%s%%20%s"
		mailto := fmt.Sprintf(mailtoFmt, testUnsubEmail, sub.Email, testUid)
		assert.Equal(t, mailto, sub.unsubMailto)

		unsubUrl := fmt.Sprintf("%s%s/%s", testUnsubBaseUrl, sub.Email, testUid)
		assert.Equal(t, unsubUrl, sub.unsubUrl)

		header := fmt.Sprintf("List-Unsubscribe: <%s>, <%s>", mailto, unsubUrl)
		assert.Equal(t, header, sub.unsubHeader)
	})

	t.Run("FillInUnsubscribeUrlReplacesTemplate", func(t *testing.T) {
		sub := setup()

		result := sub.FillInUnsubscribeUrl(
			"Unsubscribe at " + UnsubscribeUrlTemplate + " at any time",
		)

		expected := "Unsubscribe at " + sub.unsubUrl + " at any time"
		assert.Equal(t, expected, result)
	})
}
