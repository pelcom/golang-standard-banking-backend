package store

func derefStringPtr(value *string) any {
	if value == nil {
		return ""
	}
	return *value
}
