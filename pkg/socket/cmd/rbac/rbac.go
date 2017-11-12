package rbac

import (
	"fmt"
	"strings"
)

const (
	AuthCookieName = "flicktrack_io_auth_cookie"

	VIEWER_ROLE = "viewer"
	USER_ROLE   = "user"
	ADMIN_ROLE  = "admin"
)

type AuthCookieDataNs struct {
	Id    string   `json:"id"`
	Name  string   `json:"name"`
	Roles []string `json:"roles"`
}

type AuthCookieData struct {
	Namespaces []*AuthCookieDataNs `json:"namespaces"`
}

func (a *AuthCookieData) Serialize() ([]byte, error) {
	data := []string{}
	for _, ns := range a.Namespaces {
		data = append(data, fmt.Sprintf("id=%s+name=%s+roles=%s", ns.Id, ns.Name, strings.Join(ns.Roles, ",")))
	}
	return []byte(strings.Join(data, "|")), nil
}

func (a *AuthCookieData) Decode(data []byte) error {
	a.Namespaces = []*AuthCookieDataNs{}
	pipeSegs := strings.Split(string(data), "|")
	for _, pipeSeg := range pipeSegs {
		s := &AuthCookieDataNs{}
		pSegs := strings.Split(pipeSeg, "+")
		for _, pSeg := range pSegs {
			eqSegs := strings.Split(pSeg, "=")
			if len(eqSegs) < 2 {
				return malformedErr(eqSegs)
			}

			switch eqSegs[0] {
			case "id":
				s.Id = eqSegs[1]
			case "name":
				s.Name = eqSegs[1]
			case "roles":
				s.Roles = strings.Split(eqSegs[1], ",")
			default:
				return fmt.Errorf("unsupported auth-cookie key: %q", eqSegs[0])
			}
		}
		a.Namespaces = append(a.Namespaces, s)
	}

	return nil
}

func malformedErr(v interface{}) error {
	return fmt.Errorf("malformed auth-cookie key-value: %v", v)
}
