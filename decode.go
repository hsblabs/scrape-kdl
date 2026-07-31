package scrapekdl

import (
	"fmt"

	"github.com/hsblabs/scrape-kdl/internal/resultdecode"
)

// Decode strictly converts Value into destination without changing Warnings or
// Partial. Destination must be a non-nil pointer to a struct or string-keyed map.
func (result *Result) Decode(destination any) error {
	if result == nil {
		return fmt.Errorf("scrapekdl: cannot decode a nil Result")
	}
	return resultdecode.Decode(result.Value, destination)
}
