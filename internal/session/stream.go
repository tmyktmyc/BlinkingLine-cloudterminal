package session

import "encoding/json"

// streamEvent is the envelope for a single stream-json line from
// `claude --output-format stream-json`.
type streamEvent struct {
	Type    string          `json:"type"`
	Content json.RawMessage `json:"content,omitempty"`
	Delta   json.RawMessage `json:"delta,omitempty"`
	Name    string          `json:"name,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
}

// contentBlock represents one element in the content array of an
// "assistant" event.
type contentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
	Name string `json:"name,omitempty"`
}

// deltaBlock represents the delta object within a "content_block_delta" event.
type deltaBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// resultBlock represents the result object within a "result" event.
type resultBlock struct {
	Text string `json:"text,omitempty"`
}

// ParseStreamLine parses a single line of stream-json output.
// Returns assistant text extracted, tool hint name, and whether parsing succeeded.
func ParseStreamLine(line string) (assistantText string, toolHint string, ok bool) {
	var evt streamEvent
	if err := json.Unmarshal([]byte(line), &evt); err != nil {
		return "", "", false
	}

	switch evt.Type {
	case "assistant":
		// content is an array of content blocks; extract text from type=="text"
		var blocks []contentBlock
		if err := json.Unmarshal(evt.Content, &blocks); err == nil {
			for _, b := range blocks {
				if b.Type == "text" {
					assistantText += b.Text
				}
				if b.Type == "tool_use" && b.Name != "" {
					toolHint = b.Name
				}
			}
		}
		return assistantText, toolHint, true

	case "content_block_delta":
		var d deltaBlock
		if err := json.Unmarshal(evt.Delta, &d); err == nil && d.Type == "text_delta" {
			return d.Text, "", true
		}
		return "", "", true

	case "tool_use":
		return "", evt.Name, true

	case "result":
		var r resultBlock
		if err := json.Unmarshal(evt.Result, &r); err == nil {
			return r.Text, "", true
		}
		return "", "", true

	default:
		// Unknown/unhandled event type — not an error.
		return "", "", true
	}
}
