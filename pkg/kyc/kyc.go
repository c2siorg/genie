// Package kyc provides deterministic, offline validation and masking for Indian
// KYC documents — Aadhaar (UIDAI) and PAN (Income-tax Act). It never calls live
// UIDAI/NSDL services and never stores or returns a full Aadhaar number: the
// only Aadhaar value it emits is the masked last-4. This keeps the sensitive
// identifier out of logs, API responses, and the message bus by construction.
//
// Aadhaar input is the UIDAI *offline* e-KYC XML (the privacy-preserving,
// government-sanctioned path): the XML carries no full Aadhaar number, only a
// referenceId whose leading digits are the last-4. See ParseOfflineKYCLast4.
package kyc

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/protocol"
)

// DocType labels an uploaded document so the edge can apply the right handling
// and default classification.
type DocType string

const (
	DocAadhaarOfflineKYC DocType = "aadhaar_offline_kyc" // UIDAI offline e-KYC XML
	DocPAN               DocType = "pan"                 // PAN card image/PDF
	DocBankStatement     DocType = "bank_statement"      // statement CSV/PDF
	DocPassport          DocType = "passport"
	DocCSV               DocType = "csv" // the /v1/ask transaction CSV
	DocOther             DocType = "other"
)

// ValidDocType reports whether t is a recognised document type.
func ValidDocType(t DocType) bool {
	switch t {
	case DocAadhaarOfflineKYC, DocPAN, DocBankStatement, DocPassport, DocCSV, DocOther:
		return true
	default:
		return false
	}
}

// DefaultClassification returns the minimum classification a document type must
// carry. Aadhaar and passport are secret; PAN, statements and CSVs are PII. The
// upload edge must never store a sensitive type below this floor (it may only
// raise it).
func DefaultClassification(t DocType) protocol.Classification {
	switch t {
	case DocAadhaarOfflineKYC, DocPassport:
		return protocol.ClassSecret
	case DocPAN, DocBankStatement, DocCSV:
		return protocol.ClassPII
	default:
		return protocol.ClassPII
	}
}

var classRank = map[protocol.Classification]int{
	protocol.ClassPublic:   0,
	protocol.ClassInternal: 1,
	protocol.ClassPII:      2,
	protocol.ClassSecret:   3,
}

// AtLeast returns whichever of requested/floor is the more restrictive
// classification. An empty or lower requested value is raised to floor — a
// sensitive document can never be downgraded below its type's floor.
func AtLeast(requested, floor protocol.Classification) protocol.Classification {
	if classRank[requested] > classRank[floor] {
		return requested
	}
	return floor
}

// --- Aadhaar number: Verhoeff checksum -------------------------------------

// Verhoeff dihedral (D5) multiplication, permutation and inverse tables.
var verhoeffD = [10][10]int{
	{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
	{1, 2, 3, 4, 0, 6, 7, 8, 9, 5},
	{2, 3, 4, 0, 1, 7, 8, 9, 5, 6},
	{3, 4, 0, 1, 2, 8, 9, 5, 6, 7},
	{4, 0, 1, 2, 3, 9, 5, 6, 7, 8},
	{5, 9, 8, 7, 6, 0, 4, 3, 2, 1},
	{6, 5, 9, 8, 7, 1, 0, 4, 3, 2},
	{7, 6, 5, 9, 8, 2, 1, 0, 4, 3},
	{8, 7, 6, 5, 9, 3, 2, 1, 0, 4},
	{9, 8, 7, 6, 5, 4, 3, 2, 1, 0},
}

var verhoeffP = [8][10]int{
	{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
	{1, 5, 7, 6, 2, 8, 3, 0, 9, 4},
	{5, 8, 0, 3, 7, 9, 6, 1, 4, 2},
	{8, 9, 1, 6, 0, 4, 3, 5, 2, 7},
	{9, 4, 5, 3, 1, 2, 6, 8, 7, 0},
	{4, 2, 8, 6, 5, 7, 3, 9, 0, 1},
	{2, 7, 9, 3, 8, 0, 6, 4, 1, 5},
	{7, 0, 4, 6, 9, 1, 3, 2, 5, 8},
}

var verhoeffInv = [10]int{0, 4, 3, 2, 1, 5, 6, 7, 8, 9}

var digits12 = regexp.MustCompile(`^[0-9]{12}$`)

func toDigits(s string) ([]int, bool) {
	ds := make([]int, len(s))
	for i, r := range s {
		if r < '0' || r > '9' {
			return nil, false
		}
		ds[i] = int(r - '0')
	}
	return ds, true
}

// verhoeffValid reports whether the full digit sequence (payload + check digit)
// passes the Verhoeff checksum.
func verhoeffValid(ds []int) bool {
	c := 0
	for i := 0; i < len(ds); i++ {
		d := ds[len(ds)-1-i] // process right-to-left
		c = verhoeffD[c][verhoeffP[i%8][d]]
	}
	return c == 0
}

// AadhaarCheckDigit returns the Verhoeff check digit for an 11-digit payload
// (the first 11 digits of an Aadhaar). Exposed so callers/tests can construct
// or verify a full number without relying on a hard-coded example.
func AadhaarCheckDigit(first11 string) (int, error) {
	if len(first11) != 11 {
		return 0, fmt.Errorf("kyc: payload must be 11 digits, got %d", len(first11))
	}
	ds, ok := toDigits(first11)
	if !ok {
		return 0, fmt.Errorf("kyc: payload must be all digits")
	}
	c := 0
	for i := 0; i < len(ds); i++ {
		d := ds[len(ds)-1-i]
		c = verhoeffD[c][verhoeffP[(i+1)%8][d]] // check digit sits at position 0
	}
	return verhoeffInv[c], nil
}

// ValidateAadhaar reports whether s is a structurally valid Aadhaar number:
// exactly 12 digits, not beginning with 0 or 1 (reserved by UIDAI), and passing
// the Verhoeff checksum. Spaces/hyphens are tolerated. It does NOT prove the
// number was issued — only that it is well-formed.
func ValidateAadhaar(s string) bool {
	s = normalizeDigits(s)
	if !digits12.MatchString(s) {
		return false
	}
	if s[0] == '0' || s[0] == '1' {
		return false
	}
	ds, _ := toDigits(s)
	return verhoeffValid(ds)
}

// MaskAadhaar returns a masked form ("XXXX XXXX 1234") revealing only the last
// four digits, or "XXXX XXXX XXXX" if the input has fewer than 4 digits. It is
// the ONLY Aadhaar rendering the system should ever emit.
func MaskAadhaar(s string) string {
	d := normalizeDigits(s)
	if len(d) < 4 {
		return "XXXX XXXX XXXX"
	}
	return "XXXX XXXX " + d[len(d)-4:]
}

func normalizeDigits(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// --- PAN -------------------------------------------------------------------

var panRe = regexp.MustCompile(`^[A-Z]{5}[0-9]{4}[A-Z]$`)

// ValidatePAN reports whether s matches the PAN format: five letters, four
// digits, one letter (e.g. ABCDE1234F). Case-insensitive.
func ValidatePAN(s string) bool {
	return panRe.MatchString(strings.ToUpper(strings.TrimSpace(s)))
}

// --- UIDAI offline e-KYC XML ----------------------------------------------

// refIDRe pulls the referenceId out of an offline e-KYC XML. UIDAI encodes the
// Aadhaar last-4 as the first four digits of referenceId.
var refIDRe = regexp.MustCompile(`referenceId\s*=\s*"(\d{4})\d*"`)

// offlineRoots are the recognised root/element markers of a UIDAI offline
// e-KYC XML (paperless offline e-KYC). We accept either without binding to a
// full schema, since UIDAI has revised it over time.
var offlineRoots = []string{"OfflineKyc", "KycRes", "UidData", "OfflinePaperlessKyc"}

// ParseOfflineKYCLast4 validates that raw is a UIDAI offline e-KYC XML and
// returns ONLY the Aadhaar last-4 (from referenceId). It deliberately extracts
// nothing else — no name, DoB, or address — to keep this iteration to
// validation + masking. The full Aadhaar number is never present in offline
// e-KYC XML by design, so there is nothing sensitive to leak here.
func ParseOfflineKYCLast4(raw []byte) (last4 string, err error) {
	if !xmlWellFormed(raw) {
		return "", fmt.Errorf("kyc: offline e-KYC is not well-formed XML")
	}
	s := string(raw)
	if !containsAnyOfflineRoot(s) {
		return "", fmt.Errorf("kyc: does not look like a UIDAI offline e-KYC document")
	}
	m := refIDRe.FindStringSubmatch(s)
	if m == nil {
		return "", fmt.Errorf("kyc: offline e-KYC missing a referenceId with a last-4")
	}
	return m[1], nil
}

func xmlWellFormed(raw []byte) bool {
	dec := xml.NewDecoder(strings.NewReader(string(raw)))
	for {
		if _, err := dec.Token(); err != nil {
			return errors.Is(err, io.EOF)
		}
	}
}

func containsAnyOfflineRoot(s string) bool {
	for _, r := range offlineRoots {
		if strings.Contains(s, "<"+r) {
			return true
		}
	}
	return false
}
