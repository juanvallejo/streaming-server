package rbac

// Role is an object that
type Role interface {
	// AddRule composes a new Rule in the Role.
	// Returns a boolean (false) if the given rule already exists.
	AddRule(Rule) bool
	// Name returns the name assigned to the Role
	Name() string
	// Rules returns the set of rules composed by the Role
	Rules() []Rule
}

type RoleSpec struct {
	name  string
	rules []Rule
}

func (s *RoleSpec) AddRule(r Rule) bool {
	for _, rule := range s.rules {
		if r.Name() == rule.Name() {
			return false
		}
	}

	s.rules = append(s.rules, r)
	return true
}

func (s *RoleSpec) Name() string {
	return s.name
}

func (s *RoleSpec) Rules() []Rule {
	return s.rules
}

func NewRole(name string, rules []Rule) Role {
	return &RoleSpec{
		name:  name,
		rules: rules,
	}
}

type ClearRole struct {
	Role
}
