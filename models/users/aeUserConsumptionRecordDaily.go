package users

import (
	"time"
)

type AeUserConsumptionRecordDaily struct {
	Id              int64     `db:"id"`
	UserId          string    `db:"user_id"`
	Day             time.Time `db:"day"`
	XlcreditConsume float64   `db:"xlcredit_consume"`
	CreateTime      time.Time `db:"create_time"`
}
