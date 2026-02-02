package tasks

import "fmt"

func commaSeparatedString(val1, val2 string) string {
	if val1 == "" {
		if val2 != "" {
			return val2
		}
		return ""
	}

	if val2 != "" {
		return fmt.Sprintf("%s,%s", val1, val2)
	}

	return val1
}
