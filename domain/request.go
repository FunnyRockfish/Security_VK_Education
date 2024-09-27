package domain

// Структура для хранения параметров запроса
type Request struct {
	ID         string            `json:"id"`
	Method     string            `json:"method" bson:"method"`
	Path       string            `json:"path" bson:"path"`
	GetParams  map[string]string `json:"get_params" bson:"get_params"`
	Headers    map[string]string `json:"headers" bson:"headers"`
	Cookies    map[string]string `json:"cookies" bson:"cookies"`
	PostParams map[string]string `json:"post_params" bson:"post_params"`
	Body       string            `json:"body" bson:"body"`
}

// Структура для хранения ключ-значение пар
type KeyValueItem struct {
	Key   string `bson:"key"`
	Value string `bson:"value"`
}

type ReqResp struct {
	ID   string   `bson:"_id,omitempty" json:"id"`
	Req  Request  `bson:"request"`
	Resp Response `bson:"response"`
}
