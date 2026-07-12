package schedule

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Kind int

const (
	KindStar Kind = iota
	KindSingle
	KindRange
	KindStep
)

type Term struct {
	Kind             Kind
	Start, End, Step int
}

type Field struct {
	Terms  []Term
	Values []int
}

var monthNames = map[string]int{
	"jan": 1, "feb": 2, "mar": 3, "apr": 4, "may": 5, "jun": 6,
	"jul": 7, "aug": 8, "sep": 9, "oct": 10, "nov": 11, "dec": 12,
}

var weekdayNames = map[string]int{
	"sun": 0, "mon": 1, "tue": 2, "wed": 3, "thu": 4, "fri": 5, "sat": 6,
}

func parseField(field string, minVal, maxVal int, names map[string]int) (Field, error) {
	if field == "" {
		return Field{}, fmt.Errorf("empty field")
	}

	var terms []Term
	set := map[int]bool{}

	for _, term := range strings.Split(field, ",") {
		if term == "" {
			return Field{}, fmt.Errorf("empty item in %q", field)
		}

		base := term
		step := 1
		hasStep := false
		if i := strings.IndexByte(term, '/'); i >= 0 {
			base = term[:i]
			stepStr := term[i+1:]
			if base == "" || stepStr == "" {
				return Field{}, fmt.Errorf("invalid step in %q", term)
			}
			s, err := strconv.Atoi(stepStr)
			if err != nil || s < 1 {
				return Field{}, fmt.Errorf("step must be a positive number in %q", term)
			}
			step, hasStep = s, true
		}

		var lo, hi int
		isStar := false
		switch {
		case base == "*":
			lo, hi, isStar = minVal, maxVal, true

		case strings.ContainsRune(base, '-'):
			parts := strings.SplitN(base, "-", 2)
			var err error
			if lo, err = parseValue(parts[0], names); err != nil {
				return Field{}, err
			}
			if hi, err = parseValue(parts[1], names); err != nil {
				return Field{}, err
			}

		default:
			v, err := parseValue(base, names)
			if err != nil {
				return Field{}, err
			}
			lo = v
			if hasStep {
				hi = maxVal
			} else {
				hi = v
			}
		}

		if lo < minVal || hi > maxVal {
			return Field{}, fmt.Errorf("value out of range %d-%d in %q", minVal, maxVal, term)
		}
		if lo > hi {
			return Field{}, fmt.Errorf("reversed range in %q", term)
		}

		var k Kind
		switch {
		case hasStep:
			k = KindStep
		case isStar:
			k = KindStar
		case lo == hi:
			k = KindSingle
		default:
			k = KindRange
		}
		terms = append(terms, Term{Kind: k, Start: lo, End: hi, Step: step})

		for v := lo; v <= hi; v += step {
			set[v] = true
		}
	}

	values := make([]int, 0, len(set))
	for v := range set {
		values = append(values, v)
	}
	sort.Ints(values)

	return Field{Terms: terms, Values: values}, nil
}

func parseValue(s string, names map[string]int) (int, error) {
	s = strings.TrimSpace(s)
	if names != nil {
		if v, ok := names[strings.ToLower(s)]; ok {
			return v, nil
		}
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("not a number or known name: %q", s)
	}
	return v, nil
}

type Schedule struct {
	Minute Field
	Hour   Field
	Dom    Field
	Month  Field
	Dow    Field

	domRestricted bool
	dowRestricted bool
	Reboot        bool
}

func Parse(expr string) (*Schedule, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return nil, fmt.Errorf("empty expression")
	}

	if expr == "@reboot" {
		return &Schedule{Reboot: true}, nil
	}
	switch expr {
	case "@yearly", "@annually":
		expr = "0 0 1 1 *"
	case "@monthly":
		expr = "0 0 1 * *"
	case "@weekly":
		expr = "0 0 * * 0"
	case "@daily", "@midnight":
		expr = "0 0 * * *"
	case "@hourly":
		expr = "0 * * * *"
	}

	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return nil, fmt.Errorf("expected 5 fields, got %d", len(fields))
	}

	var s Schedule
	var err error

	if s.Minute, err = parseField(fields[0], 0, 59, nil); err != nil {
		return nil, fmt.Errorf("minute: %w", err)
	}
	if s.Hour, err = parseField(fields[1], 0, 23, nil); err != nil {
		return nil, fmt.Errorf("hour: %w", err)
	}
	if s.Dom, err = parseField(fields[2], 1, 31, nil); err != nil {
		return nil, fmt.Errorf("day-of-month: %w", err)
	}
	if s.Month, err = parseField(fields[3], 1, 12, monthNames); err != nil {
		return nil, fmt.Errorf("month: %w", err)
	}
	if s.Dow, err = parseField(fields[4], 0, 7, weekdayNames); err != nil {
		return nil, fmt.Errorf("day-of-week: %w", err)
	}

	s.domRestricted = fields[2] != "*"
	s.dowRestricted = fields[4] != "*"

	normalizeWeekday(&s.Dow)

	return &s, nil
}

func normalizeWeekday(f *Field) {
	seen := map[int]bool{}
	out := make([]int, 0, len(f.Values))
	for _, v := range f.Values {
		if v == 7 {
			v = 0
		}
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	sort.Ints(out)
	f.Values = out
}

func numName(v int) string { return strconv.Itoa(v) }

var monthEn = [...]string{
	1: "January", 2: "February", 3: "March", 4: "April", 5: "May", 6: "June",
	7: "July", 8: "August", 9: "September", 10: "October", 11: "November", 12: "December",
}

func monthName(v int) string {
	if v >= 1 && v <= 12 {
		return monthEn[v]
	}
	return strconv.Itoa(v)
}

var weekdayEn = [...]string{
	"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday",
}

func weekdayName(v int) string {
	if v == 7 {
		v = 0
	}
	if v >= 0 && v <= 6 {
		return weekdayEn[v]
	}
	return strconv.Itoa(v)
}

func joinEn(parts []string) string {
	switch len(parts) {
	case 0:
		return ""
	case 1:
		return parts[0]
	}
	return strings.Join(parts[:len(parts)-1], ", ") + " and " + parts[len(parts)-1]
}

func describeField(f Field, minVal, maxVal int, starPhrase, stepUnit string, name func(int) string) string {
	var parts []string
	for _, t := range f.Terms {
		switch t.Kind {
		case KindStar:
			parts = append(parts, starPhrase)
		case KindSingle:
			parts = append(parts, name(t.Start))
		case KindRange:
			parts = append(parts, name(t.Start)+" through "+name(t.End))
		case KindStep:
			p := fmt.Sprintf("every %d %s", t.Step, stepUnit)
			if t.Start != minVal || t.End != maxVal {
				p += fmt.Sprintf(" (%s through %s)", name(t.Start), name(t.End))
			}
			parts = append(parts, p)
		}
	}
	return joinEn(parts)
}

func isStar(f Field) bool {
	return len(f.Terms) == 1 && f.Terms[0].Kind == KindStar
}

func isSingle(f Field) bool {
	return len(f.Terms) == 1 && f.Terms[0].Kind == KindSingle
}

func (s *Schedule) timePhrase() string {
	mSingle := isSingle(s.Minute)
	hSingle := isSingle(s.Hour)

	if mSingle && hSingle {
		return fmt.Sprintf("at %02d:%02d", s.Hour.Values[0], s.Minute.Values[0])
	}

	var minPart string
	if mSingle {
		minPart = fmt.Sprintf("at minute %d", s.Minute.Values[0])
	} else {
		minPart = describeField(s.Minute, 0, 59, "every minute", "minutes", numName)
	}

	if isStar(s.Hour) {
		return minPart
	}

	var hrPart string
	if hSingle {
		hrPart = fmt.Sprintf("hour %d", s.Hour.Values[0])
	} else {
		hrPart = describeField(s.Hour, 0, 23, "every hour", "hours", numName)
	}
	return minPart + ", " + hrPart
}

func (s *Schedule) dayPhrase() string {
	switch {
	case s.domRestricted && s.dowRestricted:
		dom := describeField(s.Dom, 1, 31, "", "days", numName)
		dow := describeField(s.Dow, 0, 7, "", "days", weekdayName)
		return "on day-of-month " + dom + " or " + dow
	case s.domRestricted:
		return "on day-of-month " + describeField(s.Dom, 1, 31, "", "days", numName)
	case s.dowRestricted:
		return "on " + describeField(s.Dow, 0, 7, "", "days", weekdayName)
	default:
		return ""
	}
}

func (s *Schedule) Describe() string {
	if s.Reboot {
		return "at system startup"
	}

	parts := []string{s.timePhrase()}
	if day := s.dayPhrase(); day != "" {
		parts = append(parts, day)
	}
	if !isStar(s.Month) {
		parts = append(parts, "in "+describeField(s.Month, 1, 12, "every month", "months", monthName))
	}
	return strings.Join(parts, ", ")
}

func contains(vals []int, x int) bool {
	for _, v := range vals {
		if v == x {
			return true
		}
	}
	return false
}

func (s *Schedule) dayMatches(t time.Time) bool {
	dom := contains(s.Dom.Values, t.Day())
	dow := contains(s.Dow.Values, int(t.Weekday()))
	switch {
	case s.domRestricted && s.dowRestricted:
		return dom || dow
	case s.domRestricted:
		return dom
	case s.dowRestricted:
		return dow
	default:
		return true
	}
}

func (s *Schedule) Next(after time.Time) time.Time {
	if s.Reboot {
		return time.Time{}
	}

	loc := after.Location()
	t := time.Date(after.Year(), after.Month(), after.Day(), after.Hour(), after.Minute(), 0, 0, loc).Add(time.Minute)
	yearLimit := t.Year() + 5

	for {
		if t.Year() > yearLimit {
			return time.Time{}
		}

		if !contains(s.Month.Values, int(t.Month())) {
			t = time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, loc).AddDate(0, 1, 0)
			continue
		}
		if !s.dayMatches(t) {
			t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc).AddDate(0, 0, 1)
			continue
		}
		if !contains(s.Hour.Values, t.Hour()) {
			t = time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, loc).Add(time.Hour)
			continue
		}
		if !contains(s.Minute.Values, t.Minute()) {
			t = t.Add(time.Minute)
			continue
		}
		return t
	}
}
