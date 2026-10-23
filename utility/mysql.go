package utility

import (
	"errors"

	"github.com/go-sql-driver/mysql"
)

// mysqlDuplicateEntry is ER_DUP_ENTRY: an insert hit a unique key.
const mysqlDuplicateEntry = 1062

// IsDuplicateKey reports whether err is a unique key violation. It inspects the
// driver error number instead of matching message text, which changes between
// MySQL versions and locales.
func IsDuplicateKey(err error) bool {
	var mysqlErr *mysql.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == mysqlDuplicateEntry
}
