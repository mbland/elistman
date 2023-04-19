package email

import (
	"errors"
	"fmt"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
)

// This validate-before-sending algorithm:
// - https://mailtrap.io/blog/verify-email-address-without-sending/
//
// Can be implemented using:
// - https://pkg.go.dev/net/smtp

type AddressValidator interface {
	ValidateAddress(email string) error
}

type AddressValidatorImpl struct {
	From string
}

func NewValidator(fromEmail string) *AddressValidatorImpl {
	return &AddressValidatorImpl{fromEmail}
}

func (av *AddressValidatorImpl) ValidateAddress(address string) (err error) {
	user, host, err := parseUsernameAndHost(address)
	if err != nil {
		return
	}

	client, err := connectToMailHost(host)
	if err != nil {
		return
	}
	return checkMailbox(client, av.From, user, host)
}

func parseUsernameAndHost(address string) (user, host string, err error) {
	addr, parseErr := mail.ParseAddress(address)

	if parseErr != nil {
		err = fmt.Errorf(`invalid email address "%s": %s`, address, parseErr)
	} else {
		// mail.ParseAddress guarantees an "@domain" part is present.
		i := strings.LastIndexByte(addr.Address, '@')
		user = addr.Address[0:i]
		host = addr.Address[i+1:]
	}
	return
}

func connectToMailHost(hostname string) (client *smtp.Client, err error) {
	hosts, err := lookupPotentialMailHosts(hostname)

	if err != nil {
		return
	} else if client, err = tryMailHosts(hosts); err != nil {
		const errFmt = "failed to connect to a mail host for %s: %s"
		err = fmt.Errorf(errFmt, hostname, err)
	}
	return
}

func lookupPotentialMailHosts(hostname string) (hosts []string, err error) {
	errs := make([]error, 0, 3)
	hosts, err = lookupMxHosts(hostname)
	errs = append(errs, err)

	if len(hosts) == 0 {
		hosts, err = lookupSrvHosts(hostname)
		errs = append(errs, err)
	}
	if len(hosts) == 0 {
		hosts, err = lookupHosts(hostname)
		errs = append(errs, err)
	}
	if len(hosts) == 0 {
		err = errors.Join(errs...)
		err = fmt.Errorf("could not find mail host for %s: %s", hostname, err)
	}
	return
}

func lookupMxHosts(hostname string) (hosts []string, err error) {
	records, err := net.LookupMX(hostname)
	hosts = make([]string, len(records)*len(smtpPorts))
	current := 0

	for _, r := range records {
		for _, host := range appendSmtpPorts(r.Host) {
			hosts[current] = host
			current++
		}
	}
	return
}

//   - RFC 2782: A DNS RR for specifying the location of services (DNS SRV)
//     https://www.rfc-editor.org/rfc/rfc2782.html
//   - RFC 4409: Message Submission for Mail
//     https://www.rfc-editor.org/rfc/rfc4409
//   - RFC 6186: Use of SRV Records for Locating Email Submission/Access
//     Services
//     https://www.rfc-editor.org/rfc/rfc6186.html
func lookupSrvHosts(hostname string) (hosts []string, err error) {
	_, records, err := net.LookupSRV("submission", "tcp", hostname)
	hosts = make([]string, len(records))

	for i, r := range records {
		hosts[i] = fmt.Sprintf("%s:%d", r.Target, r.Port)
	}
	return
}

func lookupHosts(hostname string) (hosts []string, err error) {
	records, err := net.LookupHost(hostname)
	hosts = make([]string, 0, len(smtpPorts))

	if err != nil {
		err = errors.New("error looking up " + hostname + ": " + err.Error())
	} else if len(records) == 0 {
		err = errors.New("host not found: " + hostname)
	} else {
		hosts = append(hosts, appendSmtpPorts(hostname)...)
	}
	return
}

// https://mailtrap.io/blog/smtp-ports-25-465-587-used-for/
// https://kinsta.com/blog/smtp-port/
// https://www.mailgun.com/blog/email/which-smtp-port-understanding-ports-25-465-587/
var smtpPorts []string = []string{"587", "2525"}

func appendSmtpPorts(host string) (result []string) {
	result = make([]string, len(smtpPorts))
	i := 0

	for _, port := range smtpPorts {
		result[i] = host + ":" + port
		i++
	}
	return
}

func tryMailHosts(hosts []string) (client *smtp.Client, err error) {
	errs := make([]error, 0, len(hosts))

	for _, host := range hosts {
		if client, err = smtp.Dial(host); err == nil {
			return
		}
		err = fmt.Errorf("could not connect to %s via SMTP: %s", host, err)
		errs = append(errs, err)
	}
	err = errors.Join(errs...)
	return
}

// https://mailtrap.io/blog/verify-email-address-without-sending/
func checkMailbox(mailhost *smtp.Client, from, user, host string) (err error) {
	quitMailSession := func() error {
		if err := mailhost.Quit(); err != nil {
			return fmt.Errorf("error quitting SMTP session: %s", err)
		}
		return nil
	}
	defer func() {
		if err == nil {
			err = quitMailSession()
		} else if quitErr := quitMailSession(); quitErr != nil {
			const errFmt = "quitting SMTP session after error failed: %s\n" +
				"original error: %s"
			err = fmt.Errorf(errFmt, quitErr, err)
		}
	}()

	if err = mailhost.Hello(host); err != nil {
		return
	} else if err = mailhost.Mail(from); err != nil {
		return
	}
	return mailhost.Rcpt(user + "@" + host)
}
