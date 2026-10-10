package httpserver

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"

	"github.com/avijeet7/protomock/internal/match"
	"github.com/avijeet7/protomock/internal/models"
)

// maxClosest is how many near-miss stubs a 404 lists.
const maxClosest = 3

// noMatch is the JSON body of a 404: why no stub answered the request.
type noMatch struct {
	Error   string         `json:"error"`
	Request requestSummary `json:"request"`
	Closest []nearMiss     `json:"closest,omitempty"` // stubs for this URL, with what didn't match
	Similar []stubSummary  `json:"similar,omitempty"` // when no stub has this URL: stubs with similar URLs
}

type requestSummary struct {
	Method string `json:"method"`
	Path   string `json:"path"`
}

type stubSummary struct {
	Stub   string `json:"stub"`
	Method string `json:"method"`
	URL    string `json:"url"`
}

type nearMiss struct {
	stubSummary
	Reasons    []string `json:"reasons"`
	sameMethod bool     // used for ranking only
}

// writeNoMatch responds 404 with the closest stubs for this request and why each didn't match.
// candidates are stubs whose URL matches the path; similar (optional) suggests other stubs
// when there are none.
func writeNoMatch(w http.ResponseWriter, r *http.Request, body []byte, candidates []models.Route, similar func() []models.Route) {
	resp := noMatch{Request: requestSummary{Method: r.Method, Path: r.URL.Path}}

	if len(candidates) > 0 {
		resp.Error = fmt.Sprintf("protomock: no stub matched %s %s", r.Method, r.URL.Path)
		resp.Closest = closest(r, body, candidates)
		log.Printf("[HTTP] ❌ No stub matched %s %s — closest %s: %s",
			r.Method, r.URL.Path, resp.Closest[0].Stub, strings.Join(resp.Closest[0].Reasons, "; "))
	} else {
		resp.Error = fmt.Sprintf("protomock: no stub has URL %s", r.URL.Path)
		if similar != nil {
			for _, route := range similar() {
				resp.Similar = append(resp.Similar, summarize(route))
			}
		}
		log.Printf("[HTTP] ❌ No stub has URL %s (%s)", r.URL.Path, r.Method)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Protomock-No-Match", "true")
	w.WriteHeader(http.StatusNotFound)
	out, _ := json.MarshalIndent(resp, "", "  ")
	_, _ = w.Write(out)
}

// closest ranks candidates: stubs for the request's method first (a POST is rarely meant
// for a GET stub), then by how few rules they fail.
func closest(r *http.Request, body []byte, candidates []models.Route) []nearMiss {
	misses := make([]nearMiss, 0, len(candidates))
	for _, route := range candidates {
		misses = append(misses, nearMiss{
			stubSummary: summarize(route),
			Reasons:     mismatches(r, body, route),
			sameMethod:  matchMethod(r, route),
		})
	}
	sort.SliceStable(misses, func(i, j int) bool {
		if misses[i].sameMethod != misses[j].sameMethod {
			return misses[i].sameMethod
		}
		return len(misses[i].Reasons) < len(misses[j].Reasons)
	})
	if len(misses) > maxClosest {
		misses = misses[:maxClosest]
	}
	return misses
}

// mismatches lists the rules of route that the request fails.
func mismatches(r *http.Request, body []byte, route models.Route) []string {
	var reasons []string
	if !matchMethod(r, route) {
		reasons = append(reasons, fmt.Sprintf("method: expected %s, got %s", route.Method, r.Method))
	}
	keys := make([]string, 0, len(route.HeaderMatch))
	for k := range route.HeaderMatch {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		want, got := route.HeaderMatch[k], r.Header.Values(k)
		switch {
		case len(got) == 0:
			reasons = append(reasons, fmt.Sprintf("header %s: missing (expected %q)", k, want))
		case got[0] != want:
			reasons = append(reasons, fmt.Sprintf("header %s: expected %q, got %q", k, want, got[0]))
		}
	}
	if !matchBody(body, route) {
		reasons = append(reasons, match.ExplainJSON(route.BodyMatch, body)...)
	}
	return reasons
}

// similarRoutes suggests up to 5 stubs whose URL shares the longest prefix with the path.
func similarRoutes(r *http.Request, urlToRoutes map[string][]models.Route, regexRoutes []models.Route) []models.Route {
	var all []models.Route
	for _, routes := range urlToRoutes {
		all = append(all, routes...)
	}
	all = append(all, regexRoutes...)

	score := func(route models.Route) int {
		n := commonPrefix(route.URL, r.URL.Path)
		if route.Method == r.Method {
			n++ // prefer the same method on ties
		}
		return n
	}
	sort.SliceStable(all, func(i, j int) bool {
		si, sj := score(all[i]), score(all[j])
		if si != sj {
			return si > sj
		}
		return all[i].URL < all[j].URL
	})
	if len(all) > 5 {
		all = all[:5]
	}
	return all
}

func commonPrefix(a, b string) int {
	n := 0
	for n < len(a) && n < len(b) && a[n] == b[n] {
		n++
	}
	return n
}

func summarize(route models.Route) stubSummary {
	return stubSummary{Stub: route.Name(), Method: route.Method, URL: route.URL}
}
