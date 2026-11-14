package handler

import (
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/tahedig/too-good-to-go/backend/internal/domain/exception"
)

// queryReader parses query-string values and remembers every syntax error (wrong type or format).
// Business rules (ranges, enums, defaults) are checked in the service.
type queryReader struct {
	c    *gin.Context
	msgs []string
}

func newQueryReader(c *gin.Context) *queryReader { return &queryReader{c: c} }

func (r *queryReader) fail(name, msg string) { r.msgs = append(r.msgs, name+": "+msg) }

func (r *queryReader) err() error {
	if len(r.msgs) == 0 {
		return nil
	}
	return exception.NewBadRequest("invalid query parameters: "+strings.Join(r.msgs, "; "), nil)
}

func (r *queryReader) str(name string) string { return strings.TrimSpace(r.c.Query(name)) }

// list accepts both ?a=x,y and ?a=x&a=y.
func (r *queryReader) list(name string) []string {
	var out []string
	for _, raw := range r.c.QueryArray(name) {
		for _, part := range strings.Split(raw, ",") {
			if part = strings.TrimSpace(part); part != "" {
				out = append(out, part)
			}
		}
	}
	return out
}

func (r *queryReader) ints(name string) []int {
	var out []int
	for _, s := range r.list(name) {
		n, err := strconv.Atoi(s)
		if err != nil {
			r.fail(name, "must be a comma-separated list of integers")
			return nil
		}
		out = append(out, n)
	}
	return out
}

func (r *queryReader) intp(name string) *int {
	s := r.str(name)
	if s == "" {
		return nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		r.fail(name, "must be an integer")
		return nil
	}
	return &n
}

func (r *queryReader) int64p(name string) *int64 {
	s := r.str(name)
	if s == "" {
		return nil
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		r.fail(name, "must be an integer")
		return nil
	}
	return &n
}

func (r *queryReader) floatp(name string) *float64 {
	s := r.str(name)
	if s == "" {
		return nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
		r.fail(name, "must be a number")
		return nil
	}
	return &f
}

func (r *queryReader) timep(name string) *time.Time {
	s := r.str(name)
	if s == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		r.fail(name, "must be an RFC3339 timestamp, e.g. 2026-11-13T18:30:00+03:30")
		return nil
	}
	return &t
}
