package main

type ApiResponse struct {
	Message string `json:"message,omitempty"`
}

// PaginationInfo represents pagination metadata
type PaginationInfo struct {
	Offset int `json:"offset"`
	Size   int `json:"size"`
	Total  int `json:"total"`
}
