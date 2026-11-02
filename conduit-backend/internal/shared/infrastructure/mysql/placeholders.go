package mysql

import "strings"

func Placeholders(count int) string {
	return strings.TrimSuffix(strings.Repeat("?,", count), ",")
}

func StringArgs(values []string) []any {
	args := make([]any, len(values))
	for index, value := range values {
		args[index] = value
	}
	return args
}
