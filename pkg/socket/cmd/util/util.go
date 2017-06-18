package util

import (
	"fmt"
	"strconv"
	"strings"
	"regexp"
)

func HumanTimeToSeconds(t string) (int, error) {
	t = strings.ToLower(t)

	re := regexp.MustCompile("(\\d+(h|m|s))")
	matches := re.FindAllString(t, -1)

	if len(matches) == 0 {
		return 0, fmt.Errorf("unable to parse string... invalid format")
	}

	tsecs := 0

	for _, m := range matches {
		secs := strings.Split(m, "s")
		mins := strings.Split(m, "m")
		hrs := strings.Split(m, "h")

		if len(secs) > 1 {

			psecs, err := strconv.Atoi(secs[0])
			if err != nil {
				return 0, fmt.Errorf("unable to parse seconds rune %q", secs[0])
			}
			tsecs += psecs
			continue
		}

		if len(mins) > 1 {
			pmins, err := strconv.Atoi(mins[0])
			if err != nil {
				return 0, fmt.Errorf("unable to parse minutes rune %q", mins[0])
			}
			tsecs += pmins * 60
			continue
		}

		if len(hrs) > 1 {
			phrs, err := strconv.Atoi(hrs[0])
			if err != nil {
				return 0, fmt.Errorf("unable to parse hours rune %q", hrs[0])
			}
			tsecs += phrs * 60 * 60
			continue
		}
	}

	return tsecs, nil
}