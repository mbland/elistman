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

func (tr *TestResolver) LookupHost(
	_ context.Context, host string,
) (addrs []string, err error) {
	return tr.hosts[host], tr.hostErrs[host]
}

func (tr *TestResolver) LookupAddr(
	_ context.Context, addr string,
) (names []string, err error) {
	return tr.addrs[addr], tr.addrErrs[addr]
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
		assert.ErrorContains(t, err, `invalid email address: mblandATacm.org`)
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

func TestProcessDnsError(t *testing.T) {
	t.Run("ReturnsTrueIfIsNotFound", func(t *testing.T) {
		dnsErr := &net.DNSError{Err: "host not found", IsNotFound: true}

		err := processDnsError(dnsErr)

		assert.NilError(t, err)
	})

	t.Run("ReturnsWrappedErrorIfNotFoundIsFalse", func(t *testing.T) {
		dnsErr := &net.DNSError{Err: "DNS failure", IsNotFound: false}

		err := processDnsError(dnsErr)

		assert.ErrorContains(t, err, "DNS failure")
		assert.Assert(t, testutils.ErrorIs(err, ops.ErrExternal))
	})

	t.Run("ReturnsWrappedErrorIfNotDnsError", func(t *testing.T) {
		otherErr := errors.New("other external error")

		err := processDnsError(otherErr)

		assert.ErrorContains(t, err, "other external error")
		assert.Assert(t, testutils.ErrorIs(err, ops.ErrExternal))
	})
}

func TestLookup(t *testing.T) {
	setup := func() (*ProdAddressValidator, *TestResolver, context.Context) {
		f := newAddressValidatorFixture()
		return f.av, f.tr, f.ctx
	}

	t.Run("Succeeds", func(t *testing.T) {
		av, tr, ctx := setup()
		testHosts := []string{"foo.com", "bar.com", "baz.com"}
		tr.addrs["127.0.0.1"] = testHosts

		hosts, err := av.lookupAddr(ctx, "127.0.0.1")

		assert.NilError(t, err)
		assert.DeepEqual(t, testHosts, hosts)
	})

	t.Run("SucceedsEvenIfDnsContainsSomeBadRecords", func(t *testing.T) {
		av, tr, ctx := setup()
		testHosts := []string{"foo.com", "baz.com"}
		tr.addrs["127.0.0.1"] = testHosts
		tr.addrErrs["127.0.0.1"] = errors.New("some bad DNS records")

		hosts, err := av.lookupAddr(ctx, "127.0.0.1")

		assert.DeepEqual(t, testHosts, hosts)
		assert.Error(t, err, "error resolving 127.0.0.1: some bad DNS records")
	})

	t.Run("FailsIfNoHostsButNoResolverError", func(t *testing.T) {
		av, _, ctx := setup()

		hosts, err := av.lookupAddr(ctx, "127.0.0.1")

		assert.Equal(t, len(hosts), 0)
		assert.Error(t, err, "error resolving 127.0.0.1: no hostnames returned")
	})

	t.Run("FailsIfNoHostsAndResolverReturnsError", func(t *testing.T) {
		av, tr, ctx := setup()
		tr.addrErrs["127.0.0.1"] = errors.New("LookupAddr failed")

		hosts, err := av.lookupAddr(ctx, "127.0.0.1")

		assert.Equal(t, len(hosts), 0)
		assert.Error(t, err, "error resolving 127.0.0.1: LookupAddr failed")
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

		assert.NilError(
			t, f.av.checkHostResolvesToAddress(f.ctx, "foo.com", "172.16.0.1"),
		)
	})

	t.Run("FailsHostLookup", func(t *testing.T) {
		f := setup()
		f.tr.hosts = map[string][]string{}
		f.tr.hostErrs["foo.com"] = errors.New("test error")

		err := f.av.checkHostResolvesToAddress(f.ctx, "foo.com", "172.16.0.1")

		assert.Error(t, err, "error resolving foo.com: test error")
	})

	t.Run("FailsToResolveToAddress", func(t *testing.T) {
		f := setup()

		err := f.av.checkHostResolvesToAddress(f.ctx, "foo.com", "172.16.0.2")

		expected := "foo.com resolves to: " + strings.Join(testAddrs, ", ")
		assert.Error(t, err, expected)
	})
}

func TestCheckReverseLookupHostResolvesToOriginalIp(t *testing.T) {
	setup := func() (*ProdAddressValidator, *TestResolver, context.Context) {
		f := newAddressValidatorFixture()
		testHosts := []string{"foo.com", "bar.com", "baz.com"}
		f.tr.addrs["172.16.0.1"] = testHosts
		f.tr.addrs["172.16.0.2"] = testHosts
		f.tr.hosts["foo.com"] = []string{"127.0.0.1"}
		f.tr.hosts["bar.com"] = []string{"192.168.0.1"}
		f.tr.hosts["baz.com"] = []string{"172.16.0.1"}
		return f.av, f.tr, f.ctx
	}

	t.Run("Succeeds", func(t *testing.T) {
		v, _, ctx := setup()

		assert.NilError(
			t, v.checkReverseLookupHostResolvesToOriginalIp(ctx, "172.16.0.1"),
		)
	})

	t.Run("ReturnsAllErrorsIfNoHostResolvesToOriginalIp", func(t *testing.T) {
		// Emulates returning an error alongside valid results, and none of
		// those results resolving back to the original address.
		v, tr, ctx := setup()
		tr.addrErrs["172.16.0.2"] = errors.New("some bad DNS records")

		err := v.checkReverseLookupHostResolvesToOriginalIp(ctx, "172.16.0.2")

		expected := []string{
			"no host resolves to 172.16.0.2: " +
				"error resolving 172.16.0.2: some bad DNS records",
			"foo.com resolves to: 127.0.0.1",
			"bar.com resolves to: 192.168.0.1",
			"baz.com resolves to: 172.16.0.1",
		}
		assert.Error(t, err, strings.Join(expected, "\n"))
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

	t.Run("ReturnsAllErrorsIfAllReverseLookupsFail", func(t *testing.T) {
		av, tr, ctx := setup()
		tr.hosts["mx1.mail.foo.com"] = []string{"127.0.0.1"}
		tr.hostErrs["mx1.mail.foo.com"] = errors.New("some bad DNS records")
		tr.addrs["127.0.0.1"] = []string{"mail.foo.com", "mail.bar.com"}
		tr.hosts["mail.foo.com"] = []string{"127.0.0.2"}
		tr.hosts["mail.bar.com"] = []string{"127.0.0.3"}

		err := av.checkMailHost(ctx, "mx1.mail.foo.com")

		expected := []string{
			"reverse lookup of addresses for MX host " +
				"mx1.mail.foo.com failed: " +
				"error resolving mx1.mail.foo.com: some bad DNS records",
			"no host resolves to 127.0.0.1: " +
				"mail.foo.com resolves to: 127.0.0.2",
			"mail.bar.com resolves to: 127.0.0.3",
		}
		assert.Error(t, err, strings.Join(expected, "\n"))
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
		av, ts, _, ctx := setup()

		err := av.checkMailHosts(ctx, "foo@bar.com", "bar.com")

		expected := "error retrieving MX records for bar.com: " +
			"no records returned"
		assert.Error(t, err, expected)
		assert.Equal(t, ts.suppressedEmail, "")
	})

	t.Run("FailsAndSuppressesAddressIfMxValidationFails", func(t *testing.T) {
		av, ts, tr, ctx := setup()
		tr.mailHosts["bar.com"] = []*net.MX{{Host: "mx1.mail.bar.com"}}

		err := av.checkMailHosts(ctx, "foo@bar.com", "bar.com")

		expected := "no valid MX hosts for bar.com: " +
			"reverse lookup of addresses for MX host mx1.mail.bar.com failed"
		assert.ErrorContains(t, err, expected)
		assert.Equal(t, ts.suppressedEmail, "foo@bar.com")
	})

	t.Run("ReportsAllErrorsIncludingSuppressionError", func(t *testing.T) {
		av, ts, tr, ctx := setup()
		tr.mailHosts["bar.com"] = []*net.MX{{Host: "mx1.mail.bar.com"}}
		tr.mxErrs["bar.com"] = errors.New("some bad DNS records")
		ts.suppressErr = errors.New("suppression failed")

		err := av.checkMailHosts(ctx, "foo@bar.com", "bar.com")

		expected := []string{
			"no valid MX hosts for bar.com: " +
				"error retrieving MX records for bar.com: some bad DNS records",
			"reverse lookup of addresses for MX host mx1.mail.bar.com failed",
		}
		assert.ErrorContains(t, err, strings.Join(expected, "\n"))
		assert.ErrorContains(t, err, "suppression failed")
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

	t.Run("FailsIfAddressDoesNotParse", func(t *testing.T) {
		f := newAddressValidatorFixture()

		failure, err := f.av.ValidateAddress(f.ctx, "mblandATacm.org")

		assert.NilError(t, err)
		const expectedReason = "address failed to parse: mblandATacm.org"
		assert.Equal(t, expectedReason, failure.String())
		assert.Equal(t, "", f.ts.checkedEmail)
		assert.Equal(t, "", f.ts.suppressedEmail)
	})

	t.Run("FailsIfKnownInvalidAddress", func(t *testing.T) {
		f := newAddressValidatorFixture()

		failure, err := f.av.ValidateAddress(f.ctx, "abuse@acm.org")

		assert.NilError(t, err)
		assert.Equal(t, "invalid email address: abuse@acm.org", failure.Reason)
		assert.Equal(t, "", f.ts.checkedEmail)
		assert.Equal(t, "", f.ts.suppressedEmail)
	})

	t.Run("FailsIfAddressIsSuppressed", func(t *testing.T) {
		f := newAddressValidatorFixture()
		f.ts.isSuppressedResult = true

		failure, err := f.av.ValidateAddress(f.ctx, "mbland@acm.org")

		assert.NilError(t, err)
		const expectedReason = "suppressed email address: mbland@acm.org"
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

		failure, err := f.av.ValidateAddress(f.ctx, "mbland@acm.org")

		assert.NilError(t, err)
		const expectedReason = "address failed DNS validation: " +
			"mbland@acm.org: no valid MX hosts for acm.org: " +
			"reverse lookup of addresses for MX host " +
			"mail.mailroute.net failed: " +
			"error resolving mail.mailroute.net: no addresses returned"
		assert.Equal(t, expectedReason, failure.String())
		assert.Equal(t, "mbland@acm.org", f.ts.checkedEmail)
		assert.Equal(t, "mbland@acm.org", f.ts.suppressedEmail)
	})
}
