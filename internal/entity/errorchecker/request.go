package errorchecker

type HeadersStorage struct {
	HeadersMap map[string]string
}

func NewHeadersStorage(cookie string) *HeadersStorage {
	return &HeadersStorage{map[string]string{
		"X-User-Id":    "51523448",
		"Cookie":       cookie,
		"Content-Type": "application/json",
	}}
}
