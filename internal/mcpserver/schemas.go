package mcpserver

type memorySearchInput struct {
	Project        string   `json:"project"`
	Query          string   `json:"query"`
	Mode           string   `json:"mode"`
	Limit          int      `json:"limit"`
	Types          []string `json:"types"`
	Tags           []string `json:"tags"`
	IncludeContent bool     `json:"include_content"`
}

type memorySearchCountInput struct {
	Project string   `json:"project"`
	Query   string   `json:"query"`
	Mode    string   `json:"mode"`
	Types   []string `json:"types"`
	Tags    []string `json:"tags"`
}

type memorySearchDebugInput struct {
	Project string   `json:"project"`
	Query   string   `json:"query"`
	Mode    string   `json:"mode"`
	Limit   int      `json:"limit"`
	Types   []string `json:"types"`
	Tags    []string `json:"tags"`
}

type memoryUpsertInput struct {
	Project    string   `json:"project"`
	ID         string   `json:"id"`
	Type       string   `json:"type"`
	Title      string   `json:"title"`
	Content    string   `json:"content"`
	Tags       []string `json:"tags"`
	Importance int      `json:"importance"`
	Source     string   `json:"source"`
	Enrich     bool     `json:"enrich"`
}

type memoryDeleteInput struct {
	Project string `json:"project"`
	ID      string `json:"id"`
	Reason  string `json:"reason"`
}

type memoryGetInput struct {
	Project string `json:"project"`
	ID      string `json:"id"`
}

type memoryListInput struct {
	Project string `json:"project"`
	Type    string `json:"type"`
	Tag     string `json:"tag"`
	Limit   int    `json:"limit"`
	Offset  int    `json:"offset"`
}

type projectUpsertInput struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type memorySuggestInput struct {
	Project string `json:"project"`
	Input   string `json:"input"`
	Context string `json:"context"`
}
