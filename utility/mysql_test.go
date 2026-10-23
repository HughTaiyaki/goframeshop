package utility

import (
	"errors"
	"testing"

	"github.com/go-sql-driver/mysql"
	"github.com/gogf/gf/v2/errors/gerror"
)

func TestIsDuplicateKey(t *testing.T) {
	duplicate := &mysql.MySQLError{Number: 1062, Message: "Duplicate entry"}
	if !IsDuplicateKey(duplicate) {
		t.Fatal("bare 1062 not recognised")
	}
	// GoFrame wraps driver errors; the check must see through the wrapping.
	if !IsDuplicateKey(gerror.Wrap(duplicate, "insert")) {
		t.Fatal("wrapped 1062 not recognised")
	}
	if IsDuplicateKey(&mysql.MySQLError{Number: 1146}) || IsDuplicateKey(errors.New("1062")) || IsDuplicateKey(nil) {
		t.Fatal("non-duplicate error recognised as duplicate")
	}
}
