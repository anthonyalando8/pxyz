// internal/domain/role.go
package domain

type Role struct {
    ID          int
    Name        string // will map to role_enum
    Description string
}

type Permission struct {
    ID          int
    Name        string
    Description string
    IsAllowed   bool // true if granted, false if denied
}

// Predefined role names (must match role_enum values in DB)
const (
    RoleSystemAdmin  = "system_admin"
    RolePartnerAdmin = "partner_admin"
    RolePartnerUser  = "partner_user"
    RoleTrader       = "trader"
)

// Predefined roles list
var PredefinedRoles = []Role{
    {Name: RoleSystemAdmin, Description: "System administrator with full access"},
    {Name: RolePartnerAdmin, Description: "Partner administrator with management rights"},
    {Name: RolePartnerUser, Description: "Partner user with limited access"},
    {Name: RoleTrader, Description: "Trader with trading capabilities"},
}

// Helper: check if a role is valid
func IsValidRole(name string) bool {
    for _, r := range PredefinedRoles {
        if r.Name == name {
            return true
        }
    }
    return false
}

type RoleWithPermissions struct {
	Role        Role
	Permissions []Permission
}
