//go:build small_tests || all_tests

package email

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"

	"github.com/mbland/elistman/ops"
	"github.com/mbland/elistman/testutils"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
)

type TestResolver struct {
	mailHosts map[string][]*net.MX
	mxErrs    map[string]error
	hosts     map[string][]string
	hostErrs  map[string]error
	addrs     map[string][]string
	addrErrs  map[string]error
}

func (tr *TestResolver) LookupMX(
	_ context.Context, domain string,
) ([]*net.MX, error) {
	return tr.mailHosts[domain], tr.mxErrs[domain]
}

func (tr *TestResolver) setMxFailure(domain string, err error) {
	tr.mailHosts[domain] = []*net.MX{}
	tr.mxErrs[domain] = err
}

func (tr *TestResolver) LookupHost(
	_ context.Context, host string,
) (addrs []string, err error) {
	return tr.hosts[host], tr.hostErrs[host]
}

func (tr *TestResolver) setHostFailure(host string, err error) {
	tr.hosts[host] = []string{}
	tr.hostErrs[host] = err
}

func (tr *TestResolver) LookupAddr(
	_ context.Context, addr string,
) (names []string, err error) {
	return tr.addrs[addr], tr.addrErrs[addr]
}

func (tr *TestResolver) setAddrFailure(addr string, err error) {
	tr.addrs[addr] = []string{}
	tr.addrErrs[addr] = err
}

func assertExternalError(t *testing.T, err error) {
	t.Helper()

	assert.Assert(t, testutils.ErrorIs(err, ops.ErrExternal))
}

func assertIsNotExternalError(t *testing.T, err error) {
	t.Helper()

	assert.Assert(t, testutils.ErrorIsNot(err, ops.ErrExternal))
}

func TestParseAddress(t *testing.T) {
	t.Run("Succeeds", func(t *testing.T) {
		email, user, host, err := parseAddress("mbland@acm.org")

		assert.NilError(t, err)
		assert.Equal(t, "mbland@acm.org", email)
		assert.Equal(t, "mbland", user)
		assert.Equal(t, "acm.org", host)
	})

	t.Run("Fails", func(t *testing.T) {
		email, user, host, err := parseAddress("mblandATacm.org")

		assert.Equal(t, "", email)
		assert.Equal(t, "", user)
		assert.Equal(t, "", host)
		assert.ErrorContains(t, err, `missing '@' or angle-addr`)
		assert.ErrorContains(t, err, `missing '@'`)
	})
}

func TestGetPrimaryDomain(t *testing.T) {
	assert.Equal(
		t, "mike-bland.com", getPrimaryDomain("mike-bland.com"),
		"primary domain should remain unchanged",
	)
	assert.Equal(
		t, "mike-bland.com", getPrimaryDomain("foobar.mail.mike-bland.com"),
	)
}

func TestIsKnownInvalidAddress(t *testing.T) {
	t.Run("False", func(t *testing.T) {
		assert.Assert(t, !isKnownInvalidAddress("mbland", "acm.org"))
	})

	t.Run("TrueIfInvalidUserName", func(t *testing.T) {
		assert.Assert(t, isKnownInvalidAddress("postmaster", "acm.org"))
		assert.Assert(
			t,
			isKnownInvalidAddress("postmaster+ignore-subaddress", "acm.org"),
			"should ignore +subaddresses",
		)
	})

	t.Run("TrueIfInvalidDomain", func(t *testing.T) {
		assert.Assert(
			t,
			isKnownInvalidAddress("mbland", "[192.168.0.1]"),
			"should not allow IP address as a domain",
		)

		// Technically, I think mail.ParseAddress would flag this as an error,
		// but it pays to be paranoid on the internet.
		assert.Assert(
			t,
			isKnownInvalidAddress("mbland", "192.168.0.1"),
			"should detect IP addresses even without surrounding brackets",
		)

		assert.Assert(t, isKnownInvalidAddress("mbland", "example.com"))
		assert.Assert(
			t,
			isKnownInvalidAddress("mbland", "foobar.example.com"),
			"should detect subdomains of primary invalid domains",
		)
	})
}

func TestIsSuspiciousAddress(t *testing.T) {
	t.Run("ReturnsFalseIfNoCriteriaMet", func(t *testing.T) {
		assert.Assert(t, isSuspiciousAddress("mbland", "acm.org") == false)
	})

	t.Run("ReturnsTrueIfUsernameIsAnInt", func(t *testing.T) {
		assert.Assert(t, isSuspiciousAddress("5558675309", "acm.org") == true)
		assert.Assert(
			t, isSuspiciousAddress("jenny5558675309", "acm.org") == false,
		)
	})

	t.Run("ReturnsTrueIfEitherComponentIsAllUppercase", func(t *testing.T) {
		assert.Assert(t, isSuspiciousAddress("MBLAND", "acm.org") == true)
		assert.Assert(t, isSuspiciousAddress("mbland", "ACM.ORG") == true)
	})
}

func TestIsProblematicYetValidDomain(t *testing.T) {
	t.Run("ReturnsTrueIfInDomainsSet", func(t *testing.T) {
		assert.Assert(t, isProblematicYetValidDomain("outlook.com") == true)
	})

	t.Run("ReturnsFalseOtherwise", func(t *testing.T) {
		assert.Assert(t, isProblematicYetValidDomain("acm.org") == false)
	})
}

type addressValidatorFixture struct {
	av  *ProdAddressValidator
	ts  *TestSuppressor
	tr  *TestResolver
	ctx context.Context
}

func newAddressValidatorFixture() *addressValidatorFixture {
	resolver := &TestResolver{
		mailHosts: map[string][]*net.MX{},
		mxErrs:    map[string]error{},
		hosts:     map[string][]string{},
		hostErrs:  map[string]error{},
		addrs:     map[string][]string{},
		addrErrs:  map[string]error{},
	}
	suppressor := &TestSuppressor{}
	return &addressValidatorFixture{
		&ProdAddressValidator{suppressor, resolver},
		suppressor,
		resolver,
		context.Background(),
	}
}

func TestLookup(t *testing.T) {
	setup := func() (
		*TestResolver,
		func(context.Context, string) ([]string, error),
		context.Context,
	) {
		f := newAddressValidatorFixture()
		lookupAddr := func(_ context.Context, addr string) ([]string, error) {
			return f.tr.addrs[addr], f.tr.addrErrs[addr]
		}
		return f.tr, lookupAddr, f.ctx
	}

	t.Run("Succeeds", func(t *testing.T) {
		tr, lookupAddr, ctx := setup()
		testHosts := []string{"foo.com", "bar.com", "baz.com"}
		tr.addrs["127.0.0.1"] = testHosts

		hosts, err := lookup(lookupAddr, ctx, "127.0.0.1")

		assert.NilError(t, err)
		assert.DeepEqual(t, testHosts, hosts)
	})

	t.Run("SucceedsEvenIfDnsContainsSomeBadRecords", func(t *testing.T) {
		tr, lookupAddr, ctx := setup()
		testHosts := []string{"foo.com", "baz.com"}
		tr.addrs["127.0.0.1"] = testHosts
		tr.addrErrs["127.0.0.1"] = errors.New("some bad DNS records")

		hosts, err := lookup(lookupAddr, ctx, "127.0.0.1")

		assert.NilError(t, err)
		assert.DeepEqual(t, testHosts, hosts)
	})

	t.Run("FailsIfNoHostsFound", func(t *testing.T) {
		tr, lookupAddr, ctx := setup()
		tr.addrErrs["127.0.0.1"] = &net.DNSError{
			Err: "no such host", IsNotFound: true,
		}

		hosts, err := lookup(lookupAddr, ctx, "127.0.0.1")

		assert.Equal(t, len(hosts), 0)
		assert.Error(t, err, "no records for 127.0.0.1")
		assertIsNotExternalError(t, err)
	})

	t.Run("FailsIfExternalError", func(t *testing.T) {
		tr, lookupAddr, ctx := setup()
		tr.addrErrs["127.0.0.1"] = &net.DNSError{
			Err: "test error", IsNotFound: false,
		}

		hosts, err := lookup(lookupAddr, ctx, "127.0.0.1")

		assert.Equal(t, len(hosts), 0)
		expectedErrMsg := ops.ErrExternal.Error() +
			": failed to resolve 127.0.0.1: lookup : test error"
		assert.ErrorContains(t, err, expectedErrMsg)
		assertExternalError(t, err)
	})
}

func TestCheckHostResolvesToAddress(t *testing.T) {
	testAddrs := []string{"127.0.0.1", "192.168.0.1", "172.16.0.1"}

	setup := func() *addressValidatorFixture {
		f := newAddressValidatorFixture()
		f.tr.hosts["foo.com"] = testAddrs
		return f
	}

	t.Run("Succeeds", func(t *testing.T) {
		f := setup()

		err := f.av.checkHostResolvesToAddress(f.ctx, "foo.com", "172.16.0.1")

		assert.NilError(t, err)
	})

	t.Run("FailsHostLookup", func(t *testing.T) {
		f := setup()
		f.tr.setHostFailure("foo.com", &net.DNSError{IsNotFound: true})

		err := f.av.checkHostResolvesToAddress(f.ctx, "foo.com", "172.16.0.1")

		expectedErrMsg := "no records for foo.com"
		assert.ErrorContains(t, err, expectedErrMsg)
		assertIsNotExternalError(t, err)
	})

	t.Run("FailsToResolveToExpectedAddress", func(t *testing.T) {
		f := setup()
		addressThatDoesNotMatch := "172.16.0.2"

		err := f.av.checkHostResolvesToAddress(
			f.ctx, "foo.com", addressThatDoesNotMatch,
		)

		expectedErrMsg := "foo.com resolves to " + strings.Join(testAddrs, ", ")
		assert.ErrorContains(t, err, expectedErrMsg)
		assertIsNotExternalError(t, err)
	})

	t.Run("PassesThroughLookupError", func(t *testing.T) {
		f := setup()
		f.tr.setHostFailure(
			"foo.com", &net.DNSError{Err: "test error", IsNotFound: false},
		)

		err := f.av.checkHostResolvesToAddress(f.ctx, "foo.com", "172.16.0.1")

		expectedErrMsg := "external error: failed to resolve foo.com: " +
			"lookup : test error"
		assert.ErrorContains(t, err, expectedErrMsg)
		assertExternalError(t, err)
	})
}

func TestCheckReverseLookupHostResolvesToOriginalIp(t *testing.T) {
	const matchingAddress = "172.16.0.1"
	const addressThatDoesNotMatch = "172.16.0.2"

	setup := func() (*ProdAddressValidator, *TestResolver, context.Context) {
		f := newAddressValidatorFixture()
		testHosts := []string{"foo.com", "bar.com", "baz.com"}
		f.tr.addrs[matchingAddress] = testHosts
		f.tr.addrs[addressThatDoesNotMatch] = testHosts
		f.tr.hosts["foo.com"] = []string{"127.0.0.1"}
		f.tr.hosts["bar.com"] = []string{"192.168.0.1"}
		f.tr.hosts["baz.com"] = []string{matchingAddress}
		return f.av, f.tr, f.ctx
	}

	t.Run("Succeeds", func(t *testing.T) {
		v, _, ctx := setup()

		err := v.checkReverseLookupHostResolvesToOriginalIp(
			ctx, matchingAddress,
		)

		assert.NilError(t, err)
	})

	t.Run("ReturnsErrorIfNoHostResolvesToOriginalIp", func(t *testing.T) {
		v, _, ctx := setup()

		err := v.checkReverseLookupHostResolvesToOriginalIp(
			ctx, addressThatDoesNotMatch,
		)

		expected := []string{
			"no host resolves to 172.16.0.2: foo.com resolves to 127.0.0.1",
			"bar.com resolves to 192.168.0.1",
			"baz.com resolves to 172.16.0.1",
		}
		assert.Error(t, err, strings.Join(expected, "\n"))
		assertIsNotExternalError(t, err)
	})

	t.Run("PassesThroughAddressLookupError", func(t *testing.T) {
		v, tr, ctx := setup()
		tr.setAddrFailure(matchingAddress, errors.New("address lookup error"))

		err := v.checkReverseLookupHostResolvesToOriginalIp(
			ctx, matchingAddress,
		)

		expectedErr := "external error: failed to resolve 172.16.0.1: " +
			"address lookup error"
		assert.ErrorContains(t, err, expectedErr)
		assertExternalError(t, err)
	})

	t.Run("PassesThroughHostLookupError", func(t *testing.T) {
		v, tr, ctx := setup()
		tr.setHostFailure("bar.com", errors.New("host lookup error"))

		err := v.checkReverseLookupHostResolvesToOriginalIp(
			ctx, addressThatDoesNotMatch,
		)

		expected := []string{
			"no host resolves to 172.16.0.2: foo.com resolves to 127.0.0.1",
			"external error: failed to resolve bar.com: host lookup error",
			"baz.com resolves to 172.16.0.1",
		}
		assert.Error(t, err, strings.Join(expected, "\n"))
		assertExternalError(t, err)
	})
}

func TestCheckMailHost(t *testing.T) {
	setup := func() (*ProdAddressValidator, *TestResolver, context.Context) {
		f := newAddressValidatorFixture()
		return f.av, f.tr, f.ctx
	}

	t.Run("Succeeds", func(t *testing.T) {
		av, tr, ctx := setup()
		tr.hosts["mx1.mail.foo.com"] = []string{"127.0.0.1"}
		tr.addrs["127.0.0.1"] = []string{"mail.foo.com"}
		tr.hosts["mail.foo.com"] = []string{"127.0.0.1"}

		err := av.checkMailHost(ctx, "mx1.mail.foo.com")

		assert.NilError(t, err)
	})

	t.Run("ReturnsErrorIfAllReverseLookupsFail", func(t *testing.T) {
		av, tr, ctx := setup()
		tr.hosts["mx1.mail.foo.com"] = []string{"127.0.0.1"}
		tr.addrs["127.0.0.1"] = []string{"mail.foo.com", "mail.bar.com"}
		tr.hosts["mail.foo.com"] = []string{"127.0.0.2"}
		tr.hosts["mail.bar.com"] = []string{"127.0.0.3"}

		err := av.checkMailHost(ctx, "mx1.mail.foo.com")

		expected := []string{
			"reverse lookup of addresses for mx1.mail.foo.com failed: " +
				"no host resolves to 127.0.0.1: " +
				"mail.foo.com resolves to 127.0.0.2",
			"mail.bar.com resolves to 127.0.0.3",
		}
		assert.Error(t, err, strings.Join(expected, "\n"))
		assertIsNotExternalError(t, err)
	})

	t.Run("PassesThroughHostLookupError", func(t *testing.T) {
		av, tr, ctx := setup()
		tr.setHostFailure("mx1.mail.foo.com", errors.New("host lookup error"))

		err := av.checkMailHost(ctx, "mx1.mail.foo.com")

		expectedErrMsg := "external error: " +
			"failed to resolve mx1.mail.foo.com: host lookup error"
		assert.Error(t, err, expectedErrMsg)
		assertExternalError(t, err)
	})

	t.Run("PassesThroughAddressLookupError", func(t *testing.T) {
		av, tr, ctx := setup()
		tr.hosts["mx1.mail.foo.com"] = []string{"127.0.0.1", "127.0.0.2"}
		tr.addrs["127.0.0.1"] = []string{"mail.foo.com"}
		tr.hosts["mail.foo.com"] = []string{"127.0.0.3"}
		tr.setAddrFailure("127.0.0.2", errors.New("addr lookup failure"))

		err := av.checkMailHost(ctx, "mx1.mail.foo.com")

		expected := []string{
			"reverse lookup of addresses for mx1.mail.foo.com failed: " +
				"no host resolves to 127.0.0.1: " +
				"mail.foo.com resolves to 127.0.0.3",
			"external error: failed to resolve 127.0.0.2: addr lookup failure",
		}
		assert.Error(t, err, strings.Join(expected, "\n"))
		assertExternalError(t, err)
	})
}

func TestCheckMailHosts(t *testing.T) {
	setup := func() (
		*ProdAddressValidator, *TestSuppressor, *TestResolver, context.Context,
	) {
		f := newAddressValidatorFixture()
		return f.av, f.ts, f.tr, f.ctx
	}

	t.Run("Succeeds", func(t *testing.T) {
		av, ts, tr, ctx := setup()
		tr.mailHosts["bar.com"] = []*net.MX{{Host: "mx1.mail.bar.com"}}
		tr.hosts["mx1.mail.bar.com"] = []string{"127.0.0.1"}
		tr.addrs["127.0.0.1"] = []string{"mail.bar.com"}
		tr.hosts["mail.bar.com"] = []string{"127.0.0.1"}

		err := av.checkMailHosts(ctx, "foo@bar.com", "bar.com")

		assert.NilError(t, err)
		assert.Equal(t, ts.suppressedEmail, "")
	})

	t.Run("FailsWithoutSuppressingAddressIfNoMxRecords", func(t *testing.T) {
		av, ts, tr, ctx := setup()
		tr.setMxFailure("bar.com", errors.New("MX lookup failure"))

		err := av.checkMailHosts(ctx, "foo@bar.com", "bar.com")

		expected := "failed to retrieve MX records for bar.com: " +
			"external error: failed to resolve bar.com: MX lookup failure"
		assert.Error(t, err, expected)
		assertExternalError(t, err)
		assert.Equal(t, ts.suppressedEmail, "")
	})

	t.Run("FailsAndSuppressesAddressIfMxValidationFails", func(t *testing.T) {
		av, ts, tr, ctx := setup()
		tr.mailHosts["bar.com"] = []*net.MX{{Host: "mx1.mail.bar.com"}}
		tr.hosts["mx1.mail.bar.com"] = []string{"127.0.0.1"}
		tr.addrs["127.0.0.1"] = []string{"mail.bar.com"}

		// Make sure external errors are passed through.
		tr.setHostFailure("mail.bar.com", errors.New("host lookup failed"))

		err := av.checkMailHosts(ctx, "foo@bar.com", "bar.com")

		expected := "no valid MX hosts for bar.com: " +
			"reverse lookup of addresses for mx1.mail.bar.com failed: " +
			"no host resolves to 127.0.0.1: " +
			"external error: failed to resolve mail.bar.com: host lookup failed"
		assert.Error(t, err, expected)
		assertExternalError(t, err)
		assert.Equal(t, ts.suppressedEmail, "foo@bar.com")
	})

	t.Run("ReportsValidationAndSuppressionErrors", func(t *testing.T) {
		av, ts, tr, ctx := setup()
		tr.mailHosts["bar.com"] = []*net.MX{{Host: "mx1.mail.bar.com"}}

		// The previous test checks that external validation errors are passed
		// through. Making the validation error non-external and the suppression
		// error external enables us to validate that external suppression
		// errors are passed through as well.
		tr.setHostFailure("mx1.mail.bar.com", &net.DNSError{IsNotFound: true})
		ts.suppressErr = ops.AwsError(
			"suppression failed", testutils.AwsServerError("server error"),
		)

		err := av.checkMailHosts(ctx, "foo@bar.com", "bar.com")

		expected := []string{
			"no valid MX hosts for bar.com: no records for mx1.mail.bar.com",
			"suppression failed: external error: api error : server error",
		}
		assert.ErrorContains(t, err, strings.Join(expected, "\n"))
		assertExternalError(t, err)
		assert.Equal(t, ts.suppressedEmail, "foo@bar.com")
	})
}

func TestValidateAddress(t *testing.T) {
	t.Run("Succeeds", func(t *testing.T) {
		f := newAddressValidatorFixture()
		f.tr.mailHosts["acm.org"] = []*net.MX{{Host: "mail.mailroute.net"}}
		f.tr.hosts["mail.mailroute.net"] = []string{"199.89.3.120"}
		f.tr.addrs["199.89.3.120"] = []string{"mail.mia.mailroute.net"}
		f.tr.hosts["mail.mia.mailroute.net"] = []string{"199.89.3.120"}

		failure, err := f.av.ValidateAddress(f.ctx, "mbland@acm.org")

		assert.NilError(t, err)
		assert.Assert(t, is.Nil(failure))
		assert.Equal(t, "mbland@acm.org", f.ts.checkedEmail)
		assert.Equal(t, "", f.ts.suppressedEmail)
	})

	t.Run("SucceedsForProblematicYetValidDomain", func(t *testing.T) {
		f := newAddressValidatorFixture()
		const address = "probably-spam-but-cannot-tell-for-sure@hotmail.com"

		failure, err := f.av.ValidateAddress(f.ctx, address)

		assert.NilError(t, err)
		assert.Assert(t, is.Nil(failure))
		assert.Equal(t, address, f.ts.checkedEmail)
		assert.Equal(t, "", f.ts.suppressedEmail)
	})

	t.Run("FailsIfAddressDoesNotParse", func(t *testing.T) {
		f := newAddressValidatorFixture()

		failure, err := f.av.ValidateAddress(f.ctx, "mblandATacm.org")

		assert.NilError(t, err)
		const expectedReason = "mblandATacm.org: failed to parse"
		assert.Equal(t, expectedReason, failure.String())
		assert.Equal(t, "", f.ts.checkedEmail)
		assert.Equal(t, "", f.ts.suppressedEmail)
	})

	t.Run("FailsIfKnownInvalidAddress", func(t *testing.T) {
		f := newAddressValidatorFixture()

		failure, err := f.av.ValidateAddress(f.ctx, "abuse@acm.org")

		assert.NilError(t, err)
		assert.Equal(t, "abuse@acm.org: invalid", failure.String())
		assert.Equal(t, "", f.ts.checkedEmail)
		assert.Equal(t, "", f.ts.suppressedEmail)
	})

	t.Run("FailsIfSuspiciousAddress", func(t *testing.T) {
		f := newAddressValidatorFixture()

		failure, err := f.av.ValidateAddress(f.ctx, "MBLAND@ACM.ORG")

		assert.NilError(t, err)
		const expectedReason = "MBLAND@ACM.ORG: suspicious"
		assert.Equal(t, expectedReason, failure.String())
		assert.Equal(t, "", f.ts.checkedEmail)
		assert.Equal(t, "", f.ts.suppressedEmail)
	})

	t.Run("FailsIfAddressIsSuppressed", func(t *testing.T) {
		f := newAddressValidatorFixture()
		f.ts.isSuppressedResult = true

		failure, err := f.av.ValidateAddress(f.ctx, "mbland@acm.org")

		assert.NilError(t, err)
		const expectedReason = "mbland@acm.org: suppressed"
		assert.Equal(t, expectedReason, failure.String())
		assert.Equal(t, "mbland@acm.org", f.ts.checkedEmail)
		assert.Equal(t, "", f.ts.suppressedEmail)
	})

	t.Run("ReturnsErrorIfIsSuppressedFails", func(t *testing.T) {
		f := newAddressValidatorFixture()
		f.ts.isSuppressedErr = errors.New("unexpected SES error")

		failure, err := f.av.ValidateAddress(f.ctx, "mbland@acm.org")

		assert.Assert(t, is.Nil(failure))
		assert.ErrorContains(t, err, "unexpected SES error")
		assert.Equal(t, "mbland@acm.org", f.ts.checkedEmail)
		assert.Equal(t, "", f.ts.suppressedEmail)
	})

	t.Run("FailsIfAddressFailsDnsValidation", func(t *testing.T) {
		f := newAddressValidatorFixture()
		f.tr.mailHosts["acm.org"] = []*net.MX{{Host: "mail.mailroute.net"}}
		f.tr.setHostFailure(
			"mail.mailroute.net", &net.DNSError{IsNotFound: true},
		)

		failure, err := f.av.ValidateAddress(f.ctx, "mbland@acm.org")

		assert.NilError(t, err)
		const expectedReason = "mbland@acm.org: failed DNS validation: " +
			"no valid MX hosts for acm.org: no records for mail.mailroute.net"
		assert.Equal(t, expectedReason, failure.String())
		assert.Equal(t, "mbland@acm.org", f.ts.checkedEmail)
		assert.Equal(t, "mbland@acm.org", f.ts.suppressedEmail)
	})

	t.Run("ReturnsExternalDnsValidationError", func(t *testing.T) {
		f := newAddressValidatorFixture()
		f.tr.mailHosts["acm.org"] = []*net.MX{{Host: "mail.mailroute.net"}}
		f.tr.setHostFailure(
			"mail.mailroute.net", errors.New("host lookup failed"),
		)

		failure, err := f.av.ValidateAddress(f.ctx, "mbland@acm.org")

		assert.Assert(t, is.Nil(failure))
		const expected = "no valid MX hosts for acm.org: external error: " +
			"failed to resolve mail.mailroute.net: host lookup failed"
		assert.Error(t, err, expected)
		assertExternalError(t, err)
	})
}
