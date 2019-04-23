package route

type MetricsResponse struct {
	Invalid          int
	Published        int
	ValidationErrors map[string]ValidationError
}

type ValidationError struct {
	Count      int
	ExampleIds []int
}
