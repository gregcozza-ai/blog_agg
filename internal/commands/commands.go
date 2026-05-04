package commands

import (
	"fmt"
	"github.com/gregcozza-ai/blog_agg/internal/state"
)

type Commands struct {
	handlers map[string]func(*state.State, state.Command) error
}

func New() *Commands {
	return &Commands{
		handlers: make(map[string]func(*state.State, state.Command) error),
	}
}

func (c *Commands) Register(name string, handler func(*state.State, state.Command) error) {
	c.handlers[name] = handler
}

func (c *Commands) Run(s *state.State, cmd state.Command) error {
	handler, exists := c.handlers[cmd.Name]
	if !exists {
		return fmt.Errorf("command %q not found", cmd.Name)
	}
	return handler(s, cmd)
}

