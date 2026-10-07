package tuicast

import (
	"fmt"
	"strings"
)

// ScreenMatcher determines whether a screen has reached an expected state.
type ScreenMatcher interface {
	Match(Screen) bool
}

// ScreenMatcherFunc adapts a function into a ScreenMatcher.
type ScreenMatcherFunc func(Screen) bool

// Match calls the adapted function.
func (f ScreenMatcherFunc) Match(screen Screen) bool {
	return f(screen)
}

type describedScreenMatcher struct {
	description string
	match       func(Screen) bool
}

func (m describedScreenMatcher) Match(screen Screen) bool {
	return m.match(screen)
}

func (m describedScreenMatcher) String() string {
	return m.description
}

// ScreenContains matches when the exact screen text contains text.
func ScreenContains(text string) ScreenMatcher {
	return describedScreenMatcher{
		description: fmt.Sprintf("screen containing %q", text),
		match: func(screen Screen) bool {
			return strings.Contains(screen.Text(), text)
		},
	}
}

// ScreenLineEquals matches when row contains exactly text, including trailing
// spaces. Rows are zero-based.
func ScreenLineEquals(row int, text string) ScreenMatcher {
	return describedScreenMatcher{
		description: fmt.Sprintf("screen row %d equal to %q", row, text),
		match: func(screen Screen) bool {
			return row >= 0 && row < screen.Height && screen.Line(row) == text
		},
	}
}

// CursorAt matches the cursor's zero-based position.
func CursorAt(column, row int) ScreenMatcher {
	return describedScreenMatcher{
		description: fmt.Sprintf("cursor at column %d, row %d", column, row),
		match: func(screen Screen) bool {
			return screen.Cursor.Column == column && screen.Cursor.Row == row
		},
	}
}

// AllOf matches when every matcher succeeds. An empty AllOf matches every
// screen.
func AllOf(matchers ...ScreenMatcher) ScreenMatcher {
	return describedScreenMatcher{
		description: joinMatcherDescriptions("all of", matchers),
		match: func(screen Screen) bool {
			for _, matcher := range matchers {
				if matcher == nil || !matcher.Match(screen) {
					return false
				}
			}
			return true
		},
	}
}

// AnyOf matches when at least one matcher succeeds. An empty AnyOf never
// matches.
func AnyOf(matchers ...ScreenMatcher) ScreenMatcher {
	return describedScreenMatcher{
		description: joinMatcherDescriptions("any of", matchers),
		match: func(screen Screen) bool {
			for _, matcher := range matchers {
				if matcher != nil && matcher.Match(screen) {
					return true
				}
			}
			return false
		},
	}
}

// Not matches when matcher does not succeed.
func Not(matcher ScreenMatcher) ScreenMatcher {
	return describedScreenMatcher{
		description: "not " + matcherDescription(matcher),
		match: func(screen Screen) bool {
			return matcher != nil && !matcher.Match(screen)
		},
	}
}

func joinMatcherDescriptions(prefix string, matchers []ScreenMatcher) string {
	descriptions := make([]string, len(matchers))
	for index, matcher := range matchers {
		descriptions[index] = matcherDescription(matcher)
	}
	return fmt.Sprintf("%s (%s)", prefix, strings.Join(descriptions, ", "))
}

func matcherDescription(matcher ScreenMatcher) string {
	if matcher == nil {
		return "nil matcher"
	}
	if described, ok := matcher.(fmt.Stringer); ok {
		return described.String()
	}
	return "custom screen matcher"
}

// MatcherSpec is a serializable screen expectation used by the driver
// protocol, MCP tools, and recorded workflows. Exactly one field must be set.
type MatcherSpec struct {
	Contains *string            `json:"contains,omitempty"`
	Line     *LineMatcherSpec   `json:"line,omitempty"`
	Cursor   *CursorMatcherSpec `json:"cursor,omitempty"`
	All      []MatcherSpec      `json:"all,omitempty"`
	Any      []MatcherSpec      `json:"any,omitempty"`
	Not      *MatcherSpec       `json:"not,omitempty"`
}

// LineMatcherSpec describes ScreenLineEquals.
type LineMatcherSpec struct {
	Row  int    `json:"row"`
	Text string `json:"text"`
}

// CursorMatcherSpec describes CursorAt.
type CursorMatcherSpec struct {
	Column int `json:"column"`
	Row    int `json:"row"`
}

const maximumMatcherSpecDepth = 64

// ScreenMatcher builds the matcher described by s.
func (s MatcherSpec) ScreenMatcher() (ScreenMatcher, error) {
	return s.screenMatcherAtDepth(0)
}

func (s MatcherSpec) screenMatcherAtDepth(depth int) (ScreenMatcher, error) {
	if depth > maximumMatcherSpecDepth {
		return nil, fmt.Errorf("matcher nesting exceeds %d levels", maximumMatcherSpecDepth)
	}
	count := 0
	for _, set := range []bool{s.Contains != nil, s.Line != nil, s.Cursor != nil, s.All != nil, s.Any != nil, s.Not != nil} {
		if set {
			count++
		}
	}
	if count != 1 {
		return nil, fmt.Errorf("exactly one matcher expression is required")
	}

	switch {
	case s.Contains != nil:
		return ScreenContains(*s.Contains), nil
	case s.Line != nil:
		return ScreenLineEquals(s.Line.Row, s.Line.Text), nil
	case s.Cursor != nil:
		return CursorAt(s.Cursor.Column, s.Cursor.Row), nil
	case s.Not != nil:
		matcher, err := s.Not.screenMatcherAtDepth(depth + 1)
		if err != nil {
			return nil, err
		}
		return Not(matcher), nil
	case s.All != nil:
		matchers, err := childScreenMatchers("all", s.All, depth)
		if err != nil {
			return nil, err
		}
		return AllOf(matchers...), nil
	default:
		matchers, err := childScreenMatchers("any", s.Any, depth)
		if err != nil {
			return nil, err
		}
		return AnyOf(matchers...), nil
	}
}

func childScreenMatchers(name string, specs []MatcherSpec, depth int) ([]ScreenMatcher, error) {
	if len(specs) == 0 {
		return nil, fmt.Errorf("%s requires at least one child", name)
	}
	matchers := make([]ScreenMatcher, len(specs))
	for index, child := range specs {
		matcher, err := child.screenMatcherAtDepth(depth + 1)
		if err != nil {
			return nil, err
		}
		matchers[index] = matcher
	}
	return matchers, nil
}
