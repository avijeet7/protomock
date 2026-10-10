package match

import (
	"regexp"
	"strings"
)

// IsRegexURL reports whether a stub URL is a regex pattern rather than a literal path.
func IsRegexURL(url string) bool {
	return strings.ContainsAny(url, "*+?()[]{}")
}

// CompileURL compiles a regex stub URL, anchored so the whole path must match.
func CompileURL(url string) (*regexp.Regexp, error) {
	return regexp.Compile("^" + url + "$")
}
