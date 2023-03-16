package handler

// From https://datatracker.ietf.org/doc/html/rfc5322#section-3.2.3
//
// addr-spec       =   local-part "@" domain
// local-part      =   dot-atom / quoted-string / obs-local-part
// domain          =   dot-atom / domain-literal / obs-domain
//
// Local part
// ----------
// dot-atom-text   =   1*atext *("." atext)
// dot-atom        =   [CFWS] dot-atom-text [CFWS]
//
// obs-local-part  =   word *("." word)
// word            =   atom / quoted-string
// atom            =   [CFWS] 1*atext [CFWS]
//
// quoted-string   =   DQUOTE *([FWS] qcontent) [FWS] DQUOTE
// qcontent        =   qtext / quoted-pair
// qtext           =   %d33 /             ; Printable US-ASCII
//                     %d35-91 /          ;  characters not including
//                     %d93-126 /         ;  "\" or the quote character
//                     obs-qtext
// obs-qtext       =   obs-NO-WS-CTL
// obs-NO-WS-CTL   =   %d1-8 /            ; US-ASCII control
//                     %d11 /             ;  characters that do not
//                     %d12 /             ;  include the carriage
//                     %d14-31 /          ;  return, line feed, and
//                     %d127              ;  white space characters
//
// quoted-pair     =   ("\" (VCHAR / WSP)) / obs-qp
// obs-qp          =   "\" (%d0 / obs-NO-WS-CTL / LF / CR)
//
// Domain
// ------
// domain-literal  =   [CFWS] "[" *([FWS] dtext) [FWS] "]" [CFWS]
// dtext           =   %d33-90 /          ; Printable US-ASCII
//                     %d94-126 /         ;  characters not including
//                     obs-dtext          ;  "[", "]", or "\"
// obs-dtext       =   obs-NO-WS-CTL / quoted-pair
// obs-domain      =   atom *("." atom)
//
// From https://datatracker.ietf.org/doc/html/rfc5234#appendix-B.1
// VCHAR           =   %x21-7E            ; visible (printing) characters
// WSP             =   SP / HTAB          ; white space
// SP              =   %x20
// HTAB            =   %x09               ; horizontal tab

const atext = "[a-zA-Z0-9!#$%&'*+-/=?^_`{|}~"
const specials = "()<>[]:;@\\,.\""

func ValidateAddress(addr string) error {
	return nil
}
