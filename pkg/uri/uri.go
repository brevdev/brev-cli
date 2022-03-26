// Package uri defines a simple uri type
package uri

import (
	"fmt"
	"strings"
)

//  https://en.wikipedia.org/wiki/Uniform_Resource_Identifier .
type (
	Host string
	URL  string
)

func NewHostFromString(host string) (Host, error) {
	if strings.HasPrefix(host, "http") {
		return "", fmt.Errorf("host can not start with 'http'")
	}
	return Host(host), nil
}

func (h Host) AddPrefix(prefix string) Host {
	return Host(fmt.Sprintf("%s%s", prefix, h))
}

func (h Host) GetSlug() string {
	return strings.Split(string(h), ".")[0]
}

func (h Host) GetRootHost() string {
	domains := strings.Split(string(h), ".")
	return strings.Join([]string{domains[len(domains)-2], domains[len(domains)-1]}, ".")
}

func (h Host) ToURL() URL {
	return URL(fmt.Sprintf("https://%s", h))
}
