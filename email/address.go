package email

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/mail"
	"strings"
)

// AddressValidator wraps the ValidateAddress method.
//
// ValidateAddress parses and validates email addresses. The intent is to reduce
// bounced emails and other potential abuse by filtering out bad addresses
// before attempting to send email to them.
//
// The return value will be nil if the address passes validation, or non nil if
// it fails.
type AddressValidator interface {
	ValidateAddress(ctx context.Context, email string) error
}

// Resolver wraps several methods from the net standard library.
//
// This interface allows for unit testing code that relies on these methods
// without performing actual DNS lookups.
//
// See [net] for documentation on these methods.
type Resolver interface {
	LookupMX(ctx context.Context, name string) ([]*net.MX, error)
	LookupHost(ctx context.Context, host string) (addrs []string, err error)
	LookupAddr(ctx context.Context, addr string) (names []string, err error)
}

// ProdAddressValidator is the production implementation of AddressValidator.
type ProdAddressValidator struct {
	Suppressor Suppressor
	Resolver   Resolver
}

// ValidateAddress parses and validates email addresses.
//
// The return value will be nil if the address passes validation, or non nil if
// it fails.
//
// This method:
//
//   - Parses the username and domain with the help of [mail.ParseAddress]
//   - Rejects known invalid usernames and domains
//   - Rejects addresses on the Simple Email Service account-level suppression
//     list
//   - Looks up the DNS MX records (mail hosts) for the domain
//   - Confirms that at least one mail host is valid by examining DNS records
//
// The mail host validation happens by iterating over each MX record until one
// satisfies the following series of checks:
//
//   - Resolve the MX record's hostname to an IP address
//   - Resolve the IP address to a hostname via reverse DNS lookup (depends on a
//     DNS PTR record)
//   - Resolve that hostname to an IP address
//   - Check that two IP addresses match
//
// Each of the lookups above can produce one or more addresses or hostnames.
// ValidateAddress will iterate through every one until a match is found, or
// return an error describing all failed attempts to find a match.
//
// This algorithm was inspired by the "Reverse Entries for MX records" check
// from [DNS Inspect].
//
// Originally ValidateAddress was to implement the algorithm from [How to Verify
// Email Address Without Sending an Email].  The idea is to confirm the username
// exists for the email address domain before actually sending to it. However,
// most mail hosts these days don't allow anyone to dial in and ping mailboxes
// anymore, rendering this algorithm ineffective.
//
// DNS validation of the domain and bounce notification handling in
// [github.com/mbland/elistman/handler.Handler.HandleEvent] should minimize
// the risk of bounces and abuse.
//
// [DNS Inspect]: https://dnsinspect.com/
// [How to Verify Email Address Without Sending an Email]: https://mailtrap.io/blog/verify-email-address-without-sending/
func (av *ProdAddressValidator) ValidateAddress(
	ctx context.Context, address string,
) (err error) {
	email, user, domain, err := parseAddress(address)
	if err != nil {
		return
	} else if isKnownInvalidAddress(user, domain) {
		return errors.New("invalid email address: " + address)
	} else if av.Suppressor.IsSuppressed(email) {
		return errors.New("suppressed email address: " + email)
	}
	return av.checkMailHosts(ctx, domain)
}

func parseAddress(address string) (email, user, domain string, err error) {
	addr, parseErr := mail.ParseAddress(address)

	if parseErr != nil {
		err = fmt.Errorf("invalid email address: %s: %s", address, parseErr)
	} else {
		email = addr.Address
		// mail.ParseAddress guarantees an "@domain" part is present.
		i := strings.LastIndexByte(email, '@')
		user = email[0:i]
		domain = email[i+1:]
	}
	return
}

var invalidUserNames = map[string]bool{
	"postmaster": true,
	"abuse":      true,
}

var invalidDomains = map[string]bool{
	"localhost":   true,
	"example.com": true,
}

func isKnownInvalidAddress(user, domain string) bool {
	return invalidUserNames[strings.Split(user, "+")[0]] ||
		strings.HasPrefix(domain, "[") ||
		net.ParseIP(domain) != nil ||
		invalidDomains[domain] ||
		invalidDomains[getPrimaryDomain(domain)]
}

func getPrimaryDomain(domainName string) string {
	parts := strings.Split(domainName, ".")
	return strings.Join(parts[len(parts)-2:], ".")
}

func (av *ProdAddressValidator) checkMailHosts(
	ctx context.Context, domain string,
) error {
	mxRecords, err := av.lookupMxHosts(ctx, domain)
	const errFmt = "no valid MX hosts for %s: %s"

	if mxRecords == nil {
		return fmt.Errorf(errFmt, domain, err)
	}

	errs := make([]error, len(mxRecords))
	for i, record := range mxRecords {
		errs[i] = av.checkMailHost(ctx, record.Host)
		if errs[i] == nil {
			// Found a good MX host.
			return nil
		}
	}
	return fmt.Errorf(errFmt, domain, errors.Join(err, errors.Join(errs...)))
}

func (av *ProdAddressValidator) lookupMxHosts(
	ctx context.Context, domain string,
) ([]*net.MX, error) {
	records, err := av.Resolver.LookupMX(ctx, domain)

	if len(records) == 0 {
		if err == nil {
			err = errors.New("no MX records found")
		}
		return nil, err
	}
	return records, err
}

func (av *ProdAddressValidator) checkMailHost(
	ctx context.Context, mailHost string,
) error {
	addrs, err := av.Resolver.LookupHost(ctx, mailHost)

	if err != nil {
		return fmt.Errorf("error resolving MX host: %s: %s", mailHost, err)
	} else if len(addrs) == 0 {
		return errors.New("no addresses for MX host: " + mailHost)
	}
	return av.checkMailHostAddresses(ctx, mailHost, addrs)
}

func (av *ProdAddressValidator) checkMailHostAddresses(
	ctx context.Context, mailHost string, addrs []string,
) error {
	errs := make([]error, len(addrs))

	for i, addr := range addrs {
		errs[i] = av.checkMailHostIp(ctx, addr)
		if errs[i] == nil {
			return nil
		}
	}

	const errFmt = "reverse lookup of addresses for MX host %s failed: %s"
	return fmt.Errorf(errFmt, mailHost, errors.Join(errs...))
}

func (av *ProdAddressValidator) checkMailHostIp(
	ctx context.Context, addr string,
) error {
	hosts, err := av.Resolver.LookupAddr(ctx, addr)

	if err != nil {
		return errors.New("error resolving: " + addr)
	} else if len(hosts) == 0 {
		return errors.New("no hostnames for: " + addr)
	} else if err = av.checkHostsMatchAddress(ctx, addr, hosts); err != nil {
		const errFmt = "hosts resolved from %s don't resolve to same IP:\n%s"
		return fmt.Errorf(errFmt, addr, err)
	}
	return nil
}

func (av *ProdAddressValidator) checkHostsMatchAddress(
	ctx context.Context, addr string, hosts []string,
) error {
	errs := make([]error, len(hosts))

	for i, host := range hosts {
		addrs, err := av.Resolver.LookupHost(ctx, host)

		if err != nil {
			errs[i] = fmt.Errorf("lookup failed for: %s: %s", host, err)
			continue
		}

		errs[i] = av.checkHostMatchesAddresses(ctx, host, addrs)
		if errs[i] == nil {
			return nil
		}
	}
	return errors.Join(errs...)
}

func (av *ProdAddressValidator) checkHostMatchesAddresses(
	ctx context.Context, host string, addrs []string,
) error {
	errs := make([]error, len(addrs))

	for i, addr := range addrs {
		errs[i] = av.checkHostMatchesAddress(ctx, host, addr)
		if errs[i] == nil {
			return nil
		}
	}
	return errors.Join(errs...)
}

func (av *ProdAddressValidator) checkHostMatchesAddress(
	ctx context.Context, host, addr string,
) error {
	addrs, err := av.Resolver.LookupHost(ctx, host)

	if err != nil {
		return fmt.Errorf("error resolving %s: %s", host, err)
	}
	for _, hostAddr := range addrs {
		if hostAddr == addr {
			return nil
		}
	}
	const errFmt = "%s does not resolve to %s, resolves to: %s"
	return fmt.Errorf(errFmt, host, addr, strings.Join(addrs, ", "))
}
