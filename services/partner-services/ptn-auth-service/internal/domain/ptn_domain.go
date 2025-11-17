// domain/user.go - ADD THIS STRUCT
package domain

type PartnerUserStats struct {
	PartnerID      string
	TotalUsers     int64
	ActiveUsers    int64
	SuspendedUsers int64
	VerifiedUsers  int64
	AdminUsers     int64
	RegularUsers   int64
}