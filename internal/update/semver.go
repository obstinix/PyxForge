package update

import (
	"fmt"
	"strconv"
	"strings"
)

// Semver is a semantic version: major.minor.patch, an optional pre-release, build metadata
// dropped (it does not affect precedence).
type Semver struct {
	Major, Minor, Patch int
	Pre                 string // such as rc.2 or dev.9237c75; "" for a release
}

// ParseVersion reads a version such as 3.0.0, 3.1.0-rc.1 or v3.0.0-dev.9237c75+local.
func ParseVersion(s string) (Semver, error) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "v")
	s, _, _ = strings.Cut(s, "+")
	core, pre, _ := strings.Cut(s, "-")
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return Semver{}, fmt.Errorf("%q is not a version such as 3.0.0", s)
	}
	var n [3]int
	for i, p := range parts {
		v, err := strconv.Atoi(p)
		if err != nil || v < 0 || (len(p) > 1 && p[0] == '0') {
			return Semver{}, fmt.Errorf("%q is not a version such as 3.0.0", s)
		}
		n[i] = v
	}
	if strings.Contains(s, "-") && pre == "" {
		return Semver{}, fmt.Errorf("%q has an empty pre-release", s)
	}
	return Semver{n[0], n[1], n[2], pre}, nil
}

// Compare orders versions by SemVer 2.0 precedence: -1, 0 or 1.
func Compare(a, b Semver) int {
	for _, d := range []int{a.Major - b.Major, a.Minor - b.Minor, a.Patch - b.Patch} {
		if d != 0 {
			return sign(d)
		}
	}
	switch {
	case a.Pre == b.Pre:
		return 0
	case a.Pre == "": // a release comes after its pre-releases
		return 1
	case b.Pre == "":
		return -1
	}
	x, y := strings.Split(a.Pre, "."), strings.Split(b.Pre, ".")
	for i := 0; i < len(x) && i < len(y); i++ {
		if c := compareIdent(x[i], y[i]); c != 0 {
			return c
		}
	}
	return sign(len(x) - len(y))
}

// compareIdent orders pre-release identifiers: numbers numerically and below words, words in
// ASCII order.
func compareIdent(a, b string) int {
	an, aerr := strconv.Atoi(a)
	bn, berr := strconv.Atoi(b)
	switch {
	case aerr == nil && berr == nil:
		return sign(an - bn)
	case aerr == nil:
		return -1
	case berr == nil:
		return 1
	}
	return strings.Compare(a, b)
}

func sign(d int) int {
	switch {
	case d < 0:
		return -1
	case d > 0:
		return 1
	}
	return 0
}
