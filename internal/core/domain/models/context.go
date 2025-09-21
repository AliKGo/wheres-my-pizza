package models

type ctxKey struct{}

var requestIDKey = &ctxKey{}

func GetRequestIDKey() *ctxKey {
	return requestIDKey
}
