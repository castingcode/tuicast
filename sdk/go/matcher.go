package tuicast

// Matcher is a screen condition evaluated by the driver while waiting.
type Matcher interface {
	driverMatcher() matcherSpec
}

type matcher struct{ specification matcherSpec }

func (value matcher) driverMatcher() matcherSpec { return value.specification }

type matcherSpec struct {
	Contains *string        `json:"contains,omitempty"`
	Line     *lineMatcher   `json:"line,omitempty"`
	Cursor   *cursorMatcher `json:"cursor,omitempty"`
	All      []matcherSpec  `json:"all,omitempty"`
	Any      []matcherSpec  `json:"any,omitempty"`
	Not      *matcherSpec   `json:"not,omitempty"`
}

type lineMatcher struct {
	Row  int    `json:"row"`
	Text string `json:"text"`
}

type cursorMatcher struct {
	Column int `json:"column"`
	Row    int `json:"row"`
}

// Contains matches exact text anywhere in the complete screen text.
func Contains(text string) Matcher {
	return matcher{specification: matcherSpec{Contains: &text}}
}

// LineEquals matches one complete zero-based screen row, including trailing
// spaces.
func LineEquals(row int, text string) Matcher {
	return matcher{specification: matcherSpec{Line: &lineMatcher{Row: row, Text: text}}}
}

// CursorAt matches a zero-based cursor position.
func CursorAt(column, row int) Matcher {
	return matcher{specification: matcherSpec{Cursor: &cursorMatcher{Column: column, Row: row}}}
}

// All matches when every child matcher succeeds.
func All(matchers ...Matcher) Matcher {
	children := make([]matcherSpec, len(matchers))
	for index, child := range matchers {
		if child != nil {
			children[index] = child.driverMatcher()
		}
	}
	return matcher{specification: matcherSpec{All: children}}
}

// Any matches when at least one child matcher succeeds.
func Any(matchers ...Matcher) Matcher {
	children := make([]matcherSpec, len(matchers))
	for index, child := range matchers {
		if child != nil {
			children[index] = child.driverMatcher()
		}
	}
	return matcher{specification: matcherSpec{Any: children}}
}

// Not matches when its child matcher does not succeed.
func Not(child Matcher) Matcher {
	if child == nil {
		return matcher{}
	}
	specification := child.driverMatcher()
	return matcher{specification: matcherSpec{Not: &specification}}
}
