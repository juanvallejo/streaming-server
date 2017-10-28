package rbac

// RoleBinding links an rbac Role to a set of Subjects
type RoleBinding interface {
	// AddSubject appends a new Subject to a list of Subjects bound
	// to the RoleBinding's Role.
	// Returns a boolean (false) if the given Subject already exists
	// in the list of bound subjects, or true otherwise.
	AddSubject(Subject) bool
	// RemoveSubject removes a given subject from a list of Subjects
	// bound to the RoleBinding's Role.
	// Returns true if a Subject exists in the list of bound Subjects,
	// or false if the user is not found, or cannot be removed.
	RemoveSubject(Subject) bool
	// Role returns the role bound by the roleBinding
	Role() Role
	// Subjects returns the Subjects bound to the roleBinding
	Subjects() []Subject
}

// RoleBindingSpec implements RoleBinding
type RoleBindingSpec struct {
	roleRef  Role
	subjects []Subject
}

func (b *RoleBindingSpec) AddSubject(s Subject) bool {
	for _, sub := range b.subjects {
		if sub.UUID() == s.UUID() {
			return false
		}
	}

	b.subjects = append(b.subjects, s)
	return true
}

func (b *RoleBindingSpec) RemoveSubject(s Subject) bool {
	idxToRemove := -1

	for idx, subject := range b.subjects {
		if subject.UUID() == s.UUID() {
			idxToRemove = idx
			break
		}
	}

	if idxToRemove < 0 {
		return false
	}

	b.subjects = append(b.subjects[0:idxToRemove], b.subjects[idxToRemove+1:len(b.subjects)]...)
	return true
}

func (b *RoleBindingSpec) Role() Role {
	return b.roleRef
}

func (b *RoleBindingSpec) Subjects() []Subject {
	return b.subjects
}

// NewRoleBinding receives a Role and a slice of Subjects to
// bind to the given Role.
func NewRoleBinding(role Role, subjects []Subject) RoleBinding {
	return &RoleBindingSpec{
		roleRef:  role,
		subjects: subjects,
	}
}
