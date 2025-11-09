package version

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// Compare implements Debian's version comparison semantics as used by opkg.
// It returns -1 when a is smaller than b, 0 when they are equal and 1 when a
// is greater than b.
func Compare(a, b string) int {
	ea, ra := splitEpoch(a)
	eb, rb := splitEpoch(b)
	if ea != eb {
		if ea < eb {
			return -1
		}
		return 1
	}
	ua, da := splitRevision(ra)
	ub, db := splitRevision(rb)
	if c := comparePart(ua, ub); c != 0 {
		return c
	}
	return comparePart(da, db)
}

// CompareOp evaluates a comparison between two version strings using the
// provided operator. Supported operators match opkg's syntax: "<", "<=", "=",
// "==", ">", ">=", "<<" and ">>".
func CompareOp(a, op, b string) (bool, error) {
	switch op {
	case "<", "<=", "=", "==", ">", ">=", "<<", ">>":
	default:
		return false, fmt.Errorf("unsupported operator %q", op)
	}
	cmp := Compare(a, b)
	switch op {
	case "<", "<<":
		return cmp < 0, nil
	case "<=":
		return cmp <= 0, nil
	case "=", "==":
		return cmp == 0, nil
	case ">", ">>":
		return cmp > 0, nil
	case ">=":
		return cmp >= 0, nil
	}
	return false, nil
}

func splitEpoch(s string) (int, string) {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return 0, s
	}
	epoch, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, s
	}
	return epoch, parts[1]
}

func splitRevision(s string) (string, string) {
	if idx := strings.LastIndexByte(s, '-'); idx >= 0 {
		return s[:idx], s[idx+1:]
	}
	return s, ""
}

func comparePart(a, b string) int {
	for len(a) > 0 || len(b) > 0 {
		// Compare non-digit runs first.
		an, bn := leadingNonDigits(a), leadingNonDigits(b)
		if an != "" || bn != "" {
			if c := compareNonDigits(an, bn); c != 0 {
				return c
			}
			a = a[len(an):]
			b = b[len(bn):]
			continue
		}
		// Compare digit runs.
		ad, bd := leadingDigits(a), leadingDigits(b)
		if ad != "" || bd != "" {
			if c := compareDigits(ad, bd); c != 0 {
				return c
			}
			a = a[len(ad):]
			b = b[len(bd):]
			continue
		}
		// No digits or non-digits left (one might be empty).
		if len(a) == 0 && len(b) == 0 {
			return 0
		}
		if len(a) == 0 {
			return -1
		}
		if len(b) == 0 {
			return 1
		}
	}
	return 0
}

func leadingNonDigits(s string) string {
	i := 0
	for i < len(s) && !unicode.IsDigit(rune(s[i])) {
		i++
	}
	return s[:i]
}

func leadingDigits(s string) string {
	i := 0
	for i < len(s) && unicode.IsDigit(rune(s[i])) {
		i++
	}
	return s[:i]
}

func compareNonDigits(a, b string) int {
	ia, ib := 0, 0
	for ia < len(a) || ib < len(b) {
		ra := nextRune(a, &ia)
		rb := nextRune(b, &ib)
		oa := order(ra)
		ob := order(rb)
		if oa != ob {
			if oa < ob {
				return -1
			}
			return 1
		}
		if ra == 0 && rb == 0 {
			return 0
		}
	}
	return 0
}

func compareDigits(a, b string) int {
	a = strings.TrimLeft(a, "0")
	b = strings.TrimLeft(b, "0")
	if len(a) != len(b) {
		if len(a) < len(b) {
			return -1
		}
		return 1
	}
	if a == b {
		return 0
	}
	if a < b {
		return -1
	}
	return 1
}

func nextRune(s string, i *int) rune {
	if *i >= len(s) {
		return 0
	}
	r := rune(s[*i])
	*i = *i + 1
	return r
}

func order(r rune) int {
	switch {
	case r == 0:
		return 0
	case r == '~':
		return -1
	case unicode.IsDigit(r):
		return int(r)
	case unicode.IsLetter(r):
		return int(unicode.ToLower(r)) + 256
	default:
		return int(r) + 256
	}
}
