package rbac

// Subject has a unique identifier
type Subject interface {
	UUID() string
}

type SubjectSpec struct {
}
