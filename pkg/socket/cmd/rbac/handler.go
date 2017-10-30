package rbac

import "strings"

// Authorizer authorizes a Subject to perform an action based
// on Rules defined by Roles bound to that Subject
type Authorizer interface {
	// AddRole receives a Role and appends it to an internal list
	// of Roles.
	// Returns a boolean (false) if the given Role already exists.
	AddRole(Role) bool
	// Bind receives a Role and a set of Subjects and creates
	// a link between the Subjects and the Role.
	Bind(Role, ...Subject) bool
	// Bindings returns the role-bindings aggregated by the Authorizer
	Bindings() []RoleBinding
	// Role returns a composed Role by a given name.
	// Returns a boolean (false) if the role does not exist.
	Role(string) (Role, bool)
	// Verify verifies that a given subject has access to the
	// resources defined by the given Rule.
	// Returns a boolean (true) if the Rule given is contained
	// within the role(s) the subject has access to.
	Verify(Subject, Rule) bool
}

// AuthorizerSpec is a RoleHandler that provides several
// convenience methods for managing and restricting
// command access based on a given role.
type AuthorizerSpec struct {
	rolesByName           map[string]Role
	roleBindingByRoleName map[string]RoleBinding
}

func (a *AuthorizerSpec) AddRole(r Role) bool {
	if _, exists := a.rolesByName[r.Name()]; !exists {
		a.rolesByName[r.Name()] = r
		return true
	}

	return false
}

func (a *AuthorizerSpec) Bind(r Role, subjects ...Subject) bool {
	binding, exists := a.roleBindingByRoleName[r.Name()]
	if !exists {
		binding = NewRoleBinding(r, subjects)
		a.roleBindingByRoleName[r.Name()] = binding
	}

	for _, s := range subjects {
		binding.AddSubject(s)
	}
	return true
}

func (a *AuthorizerSpec) Bindings() []RoleBinding {
	bindings := []RoleBinding{}

	for _, b := range a.roleBindingByRoleName {
		bindings = append(bindings, b)
	}

	return bindings
}

func (a *AuthorizerSpec) Role(name string) (Role, bool) {
	if role, exists := a.rolesByName[name]; exists {
		return role, true
	}

	return nil, false
}

func (a *AuthorizerSpec) Verify(s Subject, r Rule) bool {
	subjectRoles := []Role{}

	// calculate which roles the subject is bound to
	for _, binding := range a.roleBindingByRoleName {
		found := false
		for _, subject := range binding.Subjects() {
			if subject.UUID() == s.UUID() {
				found = true
				break
			}
		}

		// given subject is bound to the role in the current binding
		if found {
			subjectRoles = append(subjectRoles, binding.Role())
		}
	}

	// iterate through the roles the given subject has been bound to
	// and calculate if at least one role contains the given rule.
	for _, role := range subjectRoles {
		for _, rule := range role.Rules() {
			if r.Name() == rule.Name() {
				return true
			}
		}
	}

	return false
}

func NewAuthorizer() Authorizer {
	return &AuthorizerSpec{
		rolesByName:           make(map[string]Role),
		roleBindingByRoleName: make(map[string]RoleBinding),
	}
}

// RuleByAction receives an action and returns the rule
// corresponding to that action, or false if no rule is found.
func RuleByAction(bindings []RoleBinding, action string) (Rule, bool) {
	for _, binding := range bindings {
		for _, rule := range binding.Role().Rules() {
			for _, a := range rule.Actions() {
				if verifyAction(a, action) {
					return rule, true
				}
			}
		}
	}
	return nil, false
}

func verifyAction(existingAction, requestedAction string) bool {
	if len(existingAction) == 0 {
		return false
	}
	if len(requestedAction) < len(existingAction) {
		return false
	}

	segsExisting := strings.Split(existingAction, "/")
	segsRequested := strings.Split(requestedAction, "/")
	for idx, segExist := range segsExisting {
		if segExist == "*" {
			return true
		}
		if idx >= len(segsRequested) {
			return false
		}
		if segExist != segsRequested[idx] {
			return false
		}
	}

	return true
}
