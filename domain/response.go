package domain

type Response struct {
	Code    int               `bson:"code"`
	Message string            `bson:"message"`
	Headers map[string]string `bson:"headers"`
	Body    string            `bson:"body"`
}
