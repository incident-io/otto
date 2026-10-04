package otto

import (
	"math/rand"
	"regexp"
	"regexp/syntax"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestRegExpFindAllMatchesStdlib checks that regExpFindAll, by both the direct
// and the reader path and with or without the context-free shortcut, finds
// the same matches as FindAllStringSubmatchIndex.
func TestRegExpFindAllMatchesStdlib(t *testing.T) {
	patterns := []string{
		`a`, `a*`, `\b\w+`, `\B`, `\b|\B`, `^a`, `(?m)^a`, `(?m)$`, `$`, `x*y|x`,
		`(a)|b`, `(?s).`, `é*`, `(?i)É`, `\w*`, `(?m)^\s*$`, `a|`, `(a*)(b*)`,
	}
	alphabet := []string{"a", "b", "x", "y", " ", "\n", "é", "É", "😀"}
	rng := rand.New(rand.NewSource(1)) //nolint:gosec // Reproducible test inputs.
	rt := New().runtime
	for _, direct := range []bool{true, false} {
		for range 10000 {
			pattern := patterns[rng.Intn(len(patterns))]
			var b strings.Builder
			for n := rng.Intn(12); n > 0; n-- {
				b.WriteString(alphabet[rng.Intn(len(alphabet))])
			}
			s := b.String()
			re := regexp.MustCompile(pattern)
			parsed, err := syntax.Parse(pattern, syntax.Perl)
			require.NoError(t, err)
			p := &regExpProgram{re: re, size: 1, contextFree: rng.Intn(2) == 0 && regExpContextFree(parsed)}
			if !direct {
				p.size = regExpDirectWork
			}
			want := re.FindAllStringSubmatchIndex(s, -1)
			require.Equal(t, want, rt.regExpFindAll(p, s, -1), "%q on %q", pattern, s)
		}
	}
}
