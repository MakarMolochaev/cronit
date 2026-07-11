package schedule

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
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
