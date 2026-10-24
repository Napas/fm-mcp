package mcpserver

import (
	"reflect"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Napas/fm-mcp/internal/fm"
)

// schemaOverrides describe types whose JSON differs from their Go shape.
var schemaOverrides = map[reflect.Type]*jsonschema.Schema{
	// fm.Attributes is an array in Go and a name → value object in JSON.
	reflect.TypeFor[fm.Attributes](): {
		Type:                 "object",
		Description:          "attribute name → value on the 1-20 scale; attributes fog of war hides are left out",
		AdditionalProperties: &jsonschema.Schema{Type: "integer"},
	},
}

// addTool registers a tool whose output schema has schemaOverrides applied,
// so outputs containing fm.Attributes validate.
func addTool[In, Out any](srv *mcp.Server, t *mcp.Tool, h mcp.ToolHandlerFor[In, Out]) {
	if t.OutputSchema == nil {
		t.OutputSchema = outputSchema[Out]()
	}
	mcp.AddTool(srv, t, h)
}

// outputSchema infers T's schema with schemaOverrides applied. Pointers are
// dereferenced first: MCP requires an output schema of type "object", and
// jsonschema-go makes pointer types nullable.
func outputSchema[T any]() *jsonschema.Schema {
	rt := reflect.TypeFor[T]()
	if rt.Kind() == reflect.Pointer {
		rt = rt.Elem()
	}
	s, err := jsonschema.ForType(rt, &jsonschema.ForOptions{TypeSchemas: schemaOverrides})
	if err != nil {
		panic(err) // the types are fixed at compile time
	}
	return s
}
