package rbac

import (
	"encoding/json"
)

const (
	AuthCookieName = "flicktrack.io/auth/cookie"

	VIEWER_ROLE = "viewer"
	USER_ROLE   = "user"
	ADMIN_ROLE  = "admin"
)

type SerializableRole interface {
	Serialize() ([]byte, error)
}

type SerializableRoleSpec struct {
	Id   string `json:"id"`
	Role string `json:"role"`
}

func (s *SerializableRoleSpec) Serialize() ([]byte, error) {
	return json.Marshal(s)
}

func NewSerializableRole(id, role string) SerializableRole {
	return &SerializableRoleSpec{
		Id:   id,
		Role: role,
	}
}
