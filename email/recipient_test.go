//go:build small_tests || all_tests

package email

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/mbland/elistman/ops"
	tu "github.com/mbland/elistman/testutils"
	"gotest.tools/assert"
)

func TestSubscriber(t *testing.T) {
	setup := func() *Recipient {
		sub := &Recipient{
			Email: "subscriber@foo.com",
			Uid:   uuid.MustParse(testUid),
		}
		sub.SetUnsubscribeInfo(testUnsubEmail, testApiBaseUrl)
		return sub
	}

	expectedUrlAndHeader := func(sub *Recipient) (string, string) {
		const mailtoFmt = "mailto:%s?subject=%s%%20%s"
		mailto := fmt.Sprintf(mailtoFmt, testUnsubEmail, sub.Email, testUid)
		unsubUrl := testApiBaseUrl + ops.ApiPrefixUnsubscribe +
			sub.Email + "/" + testUid
		header := fmt.Sprintf(
			"List-Unsubscribe: <%s>, <%s>\r\n", mailto, unsubUrl,
		)
		return unsubUrl, header
	}

	t.Run("SetUnsubscribeInfoSetsPrivateUnsubFields", func(t *testing.T) {
		sub := setup()

		unsubUrl, header := expectedUrlAndHeader(sub)
		assert.Equal(t, unsubUrl, string(sub.unsubUrl))
		assert.Equal(t, header, string(sub.unsubHeader))
	})

	t.Run("FillInUnsubscribeUrlReplacesTemplate", func(t *testing.T) {
		sub := setup()
		orig := "Unsubscribe at " + UnsubscribeUrlTemplate + " at any time"

		result := sub.FillInUnsubscribeUrl([]byte(orig))

		expected := "Unsubscribe at " + string(sub.unsubUrl) + " at any time"
		assert.Equal(t, expected, string(result))
	})

	t.Run("EmitUnsubscribeHeaders", func(t *testing.T) {
		emitHeadersSetup := func() (
			*Recipient, *strings.Builder, *tu.ErrWriter,
		) {
			sb := &strings.Builder{}
			return setup(), sb, &tu.ErrWriter{Buf: sb}
		}

		t.Run("EmitsNothingIfUnsubInfoNotSet", func(t *testing.T) {
			sub, w, _ := emitHeadersSetup()
			sub.unsubHeader = []byte{}
			sub.unsubUrl = []byte{}

			err := sub.EmitUnsubscribeHeaders(w)

			assert.NilError(t, err)
			assert.Equal(t, "", w.String())
		})

		t.Run("EmitsIfUnsubInfoSet", func(t *testing.T) {
			sub, w, _ := emitHeadersSetup()

			err := sub.EmitUnsubscribeHeaders(w)

			assert.NilError(t, err)
			_, headers := expectedUrlAndHeader(sub)
			headers += string(listUnsubscribePost)
			assert.Equal(t, headers, w.String())
		})

		t.Run("ReturnsErrorFromWritingFirstHeader", func(t *testing.T) {
			sub, _, ew := emitHeadersSetup()
			ew.ErrorOn = "List-Unsubscribe: "
			ew.Err = errors.New("write error")

			assert.Error(t, sub.EmitUnsubscribeHeaders(ew), "write error")
		})

		t.Run("ReturnsErrorFromWritingSecondHeader", func(t *testing.T) {
			sub, _, ew := emitHeadersSetup()
			ew.ErrorOn = "List-Unsubscribe-Post: "
			ew.Err = errors.New("write error")

			assert.Error(t, sub.EmitUnsubscribeHeaders(ew), "write error")
		})
	})
}
