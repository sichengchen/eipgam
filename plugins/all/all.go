// Package all registers every built-in plugin. Only eipgam's main imports
// it: core never imports a plugin, and a plugin never imports another.
package all

import (
	_ "github.com/sichengchen/eipgam/plugins/budget"
	_ "github.com/sichengchen/eipgam/plugins/gateway"
	_ "github.com/sichengchen/eipgam/plugins/groups"
	_ "github.com/sichengchen/eipgam/plugins/library"
	_ "github.com/sichengchen/eipgam/plugins/middleware"
	_ "github.com/sichengchen/eipgam/plugins/otel"
	_ "github.com/sichengchen/eipgam/plugins/redact"
	_ "github.com/sichengchen/eipgam/plugins/rotation"
	_ "github.com/sichengchen/eipgam/plugins/routing"
	_ "github.com/sichengchen/eipgam/plugins/search"
	_ "github.com/sichengchen/eipgam/plugins/sessions"
	_ "github.com/sichengchen/eipgam/plugins/tools"
	_ "github.com/sichengchen/eipgam/plugins/usage"
	_ "github.com/sichengchen/eipgam/plugins/warmups"
)
