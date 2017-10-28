package rbac

// Rule defines a permission
type Rule interface {
	// Name returns the name associated with the Rule
	Name() string
	// Actions returns the specific set of actions for which the Rule allows access
	Actions() []string
}

// RuleSpec implements Rule
type RuleSpec struct {
	actions []string
	name    string
}

func (r *RuleSpec) Name() string {
	return r.name
}

func (r *RuleSpec) Actions() []string {
	return r.actions
}

func NewRule(name string, actions []string) Rule {
	return &RuleSpec{
		name:    name,
		actions: actions,
	}
}
