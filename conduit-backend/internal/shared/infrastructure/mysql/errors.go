package mysql

import (
	"errors"
	"regexp"
	"strings"

	driver "github.com/go-sql-driver/mysql"
)

const duplicateEntryNumber = 1062

var duplicateKeyPattern = regexp.MustCompile(`for key '([^']+)'$`)

func IsUniqueViolation(err error) bool {
	var mysqlErr *driver.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == duplicateEntryNumber
}

func ConstraintName(err error) string {
	var mysqlErr *driver.MySQLError
	if !errors.As(err, &mysqlErr) || mysqlErr.Number != duplicateEntryNumber {
		return ""
	}
	match := duplicateKeyPattern.FindStringSubmatch(mysqlErr.Message)
	if match == nil {
		return ""
	}
	name := match[1]
	if dot := strings.LastIndex(name, "."); dot >= 0 {
		name = name[dot+1:]
	}
	return name
}
