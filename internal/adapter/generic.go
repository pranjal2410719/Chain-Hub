package adapter

// GenericAdapter is a general-purpose adapter that can wrap ANY CLI tool.  It
// is used by the plugin loader to create adapters from YAML manifest files,
// allowing users to add custom tools to ChainHub without writing Go code.
type GenericAdapter struct {
	*BaseAdapter
}

// NewGenericAdapter creates a GenericAdapter from the supplied ToolInfo.  The
// caller (typically the plugin loader) is responsible for populating all fields
// of the ToolInfo struct.
func NewGenericAdapter(info ToolInfo) *GenericAdapter {
	return &GenericAdapter{
		BaseAdapter: NewBaseAdapter(info),
	}
}
