package rules

import (
	"time"

	"github.com/nvat/tgifreezeday/internal/core/domain"
)

// Engine processes rules to determine freeze days
type Engine struct {
	rules []domain.Rule
}

// NewEngine creates a new rule engine
func NewEngine() *Engine {
	return &Engine{
		rules: make([]domain.Rule, 0),
	}
}

// AddRule adds a rule to the engine
func (e *Engine) AddRule(r domain.Rule) {
	e.rules = append(e.rules, r)
}

// IsFreezePeriod checks if a date is within a freeze period based on defined rules
func (e *Engine) IsFreezePeriod(t time.Time) (bool, string) {
	for _, rule := range e.rules {
		if rule.Apply(t) {
			return true, rule.Description()
		}
	}
	return false, ""
}
