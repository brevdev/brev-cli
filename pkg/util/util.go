package util

import (
	"fmt"
)

// This package should only be used as a holding pattern to be later moved into more specific packages

func MapAppend(m map[string]interface{}, n ...map[string]interface{}) map[string]interface{} {
	if m == nil { // we may get nil maps from legacy users not having user.OnboardingStatus set
		m = make(map[string]interface{})
	}
	for _, item := range n {
		for key, value := range item {
			m[key] = value
		}
	}
	return m
}

// checks if noun or pulural version of noun (checks if s at end)
func IsSingularOrPlural(check, noun string) bool {
	// TODO complex logic
	return check == noun || fmt.Sprintf("%ss", noun) == check
}
