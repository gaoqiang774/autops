package dao

import (
	"testing"

	"github.com/go-sql-driver/mysql"
)

func TestIsSysAdminUsernameDuplicateError(t *testing.T) {
	if !isSysAdminUsernameDuplicateError(&mysql.MySQLError{
		Number: 1062, Message: "Duplicate entry 'zhangsan' for key 'username'",
	}) {
		t.Fatal("username unique-key collision should be recognized")
	}
	if isSysAdminUsernameDuplicateError(&mysql.MySQLError{
		Number: 1062, Message: "Duplicate entry '1' for key 'other_key'",
	}) {
		t.Fatal("unrelated unique-key collision should not be reported as a username conflict")
	}
}
