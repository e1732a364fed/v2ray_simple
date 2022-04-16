package httpLayer

//被用于 proxy的 CommonConf
type HeaderConf struct {
	Request *struct {
		Version string
		Method  string
		Path    []string
		Headers map[string][]string
	}

	Response *struct {
		Version string
		Status  string
		Reason  string
		Headers map[string][]string
	}
}
