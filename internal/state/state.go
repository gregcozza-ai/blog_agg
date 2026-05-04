package state

import (
	"github.com/gregcozza-ai/blog_agg/internal/config"
	"github.com/gregcozza-ai/blog_agg/internal/database"
)

type State struct {
	Db	*database.Queries
	Cfg 	*config.Config
}

type Command struct {
	Name	string
	Args	[]string
}

