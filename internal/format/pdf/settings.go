package pdf

// Reading the settings a request carries: how many pages and which paper.

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	defaultPages = 1
	maxPages     = 5000
)

type pageSize struct {
	name          string
	width, height int
}

var pageSizes = map[string]pageSize{
	"a4":     {"A4", 595, 842},
	"a3":     {"A3", 842, 1191},
	"a5":     {"A5", 420, 595},
	"letter": {"Letter", 612, 792},
	"legal":  {"Legal", 612, 1008},
}

func pageCount(props map[string]string) (int, error) {
	raw, ok := props["pages"]
	if !ok || raw == "" {
		return defaultPages, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("pdf: pages must be a whole number, got %q", raw)
	}
	if n < 1 || n > maxPages {
		return 0, fmt.Errorf("pdf: pages must be between 1 and %d, got %d", maxPages, n)
	}
	return n, nil
}

func paperSize(props map[string]string) (pageSize, error) {
	raw, ok := props["page_size"]
	if !ok || raw == "" {
		return pageSizes["a4"], nil
	}
	s, ok := pageSizes[strings.ToLower(raw)]
	if !ok {
		names := make([]string, 0, len(pageSizes))
		for k := range pageSizes {
			names = append(names, k)
		}
		return pageSize{}, fmt.Errorf("pdf: page_size %q is not one of: %s", raw, strings.Join(sorted(names), ", "))
	}
	return s, nil
}

func sorted(in []string) []string {
	out := append([]string(nil), in...)
	for i := range out {
		for j := i + 1; j < len(out); j++ {
			if out[j] < out[i] {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}
