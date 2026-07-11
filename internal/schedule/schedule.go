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
		return Field{}, fmt.Errorf("пустое поле")
	}

	var terms []Term
	set := map[int]bool{}

	for _, term := range strings.Split(field, ",") {
		if term == "" {
			return Field{}, fmt.Errorf("пустой элемент в %q", field)
		}

		base := term
		step := 1
		hasStep := false
		if i := strings.IndexByte(term, '/'); i >= 0 {
			base = term[:i]
			stepStr := term[i+1:]
			if base == "" || stepStr == "" {
				return Field{}, fmt.Errorf("некорректный шаг в %q", term)
			}
			s, err := strconv.Atoi(stepStr)
			if err != nil || s < 1 {
				return Field{}, fmt.Errorf("шаг должен быть положительным числом в %q", term)
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
			return Field{}, fmt.Errorf("значение вне диапазона %d-%d в %q", minVal, maxVal, term)
		}
		if lo > hi {
			return Field{}, fmt.Errorf("перевёрнутый диапазон в %q", term)
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
		return 0, fmt.Errorf("не число и не известное имя: %q", s)
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
		return nil, fmt.Errorf("пустое выражение")
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
		return nil, fmt.Errorf("ожидалось 5 полей, получено %d", len(fields))
	}

	var s Schedule
	var err error

	if s.Minute, err = parseField(fields[0], 0, 59, nil); err != nil {
		return nil, fmt.Errorf("минуты: %w", err)
	}
	if s.Hour, err = parseField(fields[1], 0, 23, nil); err != nil {
		return nil, fmt.Errorf("часы: %w", err)
	}
	if s.Dom, err = parseField(fields[2], 1, 31, nil); err != nil {
		return nil, fmt.Errorf("день месяца: %w", err)
	}
	if s.Month, err = parseField(fields[3], 1, 12, monthNames); err != nil {
		return nil, fmt.Errorf("месяц: %w", err)
	}
	if s.Dow, err = parseField(fields[4], 0, 7, weekdayNames); err != nil {
		return nil, fmt.Errorf("день недели: %w", err)
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

var monthRu = [...]string{
	1: "январь", 2: "февраль", 3: "март", 4: "апрель", 5: "май", 6: "июнь",
	7: "июль", 8: "август", 9: "сентябрь", 10: "октябрь", 11: "ноябрь", 12: "декабрь",
}

func monthName(v int) string {
	if v >= 1 && v <= 12 {
		return monthRu[v]
	}
	return strconv.Itoa(v)
}

var weekdayRu = [...]string{
	"воскресенье", "понедельник", "вторник", "среда", "четверг", "пятница", "суббота",
}

func weekdayName(v int) string {
	if v == 7 {
		v = 0
	}
	if v >= 0 && v <= 6 {
		return weekdayRu[v]
	}
	return strconv.Itoa(v)
}

func joinRu(parts []string) string {
	switch len(parts) {
	case 0:
		return ""
	case 1:
		return parts[0]
	}
	return strings.Join(parts[:len(parts)-1], ", ") + " и " + parts[len(parts)-1]
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
			parts = append(parts, "с "+name(t.Start)+" по "+name(t.End))
		case KindStep:
			p := fmt.Sprintf("каждые %d %s", t.Step, stepUnit)
			if t.Start != minVal || t.End != maxVal {
				p += " (диапазон " + name(t.Start) + "-" + name(t.End) + ")"
			}
			parts = append(parts, p)
		}
	}
	return joinRu(parts)
}

func isStar(f Field) bool {
	return len(f.Terms) == 1 && f.Terms[0].Kind == KindStar
}

func (s *Schedule) describeDays() string {
	switch {
	case s.domRestricted && s.dowRestricted:
		dom := describeField(s.Dom, 1, 31, "", "дн", numName)
		dow := describeField(s.Dow, 0, 7, "", "дн", weekdayName)
		return "дни: " + dom + " ИЛИ " + dow
	case s.domRestricted:
		return "дни месяца: " + describeField(s.Dom, 1, 31, "", "дн", numName)
	case s.dowRestricted:
		return "дни недели: " + describeField(s.Dow, 0, 7, "", "дн", weekdayName)
	default:
		return "каждый день"
	}
}

func (s *Schedule) Describe() string {
	if s.Reboot {
		return "при загрузке системы"
	}

	minutes := describeField(s.Minute, 0, 59, "каждую минуту", "мин", numName)
	hours := describeField(s.Hour, 0, 23, "каждый час", "ч", numName)

	clauses := []string{
		"минуты: " + minutes,
		"часы: " + hours,
		s.describeDays(),
	}
	if !isStar(s.Month) {
		clauses = append(clauses, "месяцы: "+describeField(s.Month, 1, 12, "каждый месяц", "мес", monthName))
	}
	return strings.Join(clauses, "; ")
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
