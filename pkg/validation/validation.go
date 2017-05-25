package validation

import (
	"fmt"
	"regexp"
)

var ClientValidationPattern string = "^[a-zA-Z_0-9]+$"
var ClientValidation *regexp.Regexp

// ValidateClientUsername receives a username and returns an error if it does not comply
// with ClientValidationPattern
func ValidateClientUsername(name string) error {
	if !ClientValidation.MatchString(name) {
		return fmt.Errorf("error: username %q is invalid", name)
	}

	return nil
}

func init() {
	var err error
	ClientValidation, err = regexp.Compile(ClientValidationPattern)
	if err != nil {
		panic(fmt.Sprintf("error compiling username validation pattern %q", ClientValidationPattern))
	}
}
