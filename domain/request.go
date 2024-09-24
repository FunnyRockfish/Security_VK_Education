package domain

type Request struct {
	Method     string `json:"method"`
	Path       string `json:"path"`
	GetParams  []KeyValueItem
	Headers    []KeyValueItem
	Cookies    []KeyValueItem
	PostParams []KeyValueItem
}

type KeyValueItem struct {
	Key   string
	Value string
}
