package mcp

import "encoding/json"

// MCPTool wraps an MCP server tool as an axe Tool
type MCPTool struct {
	client      *Client
	name        string
	description string
	schema      any
}

func (t *MCPTool) Name() string        { return t.name }
func (t *MCPTool) Description() string { return t.description }
func (t *MCPTool) Schema() any         { return t.schema }
func (t *MCPTool) Execute(input json.RawMessage) (string, error) {
	return t.client.CallTool(t.name, input)
}

// Tools returns all MCP server tools as axe-compatible Tool interfaces
func (c *Client) Tools() []MCPTool {
	infos, err := c.ListTools()
	if err != nil {
		return nil
	}
	tools := make([]MCPTool, len(infos))
	for i, info := range infos {
		tools[i] = MCPTool{client: c, name: info.Name, description: info.Description, schema: info.InputSchema}
	}
	return tools
}
