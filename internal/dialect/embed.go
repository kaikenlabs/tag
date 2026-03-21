package dialect

import "embed"

//go:embed dialects/*.yaml
var builtinFS embed.FS
