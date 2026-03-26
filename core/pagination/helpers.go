package pagination

import (
	paginationpb "github.com/Servora-Kit/servora/api/gen/go/servora/pagination/v1"
)

const (
	DefaultPage     int32 = 1
	DefaultPageSize int32 = 20
)

// ExtractPage extracts page and pageSize from a PaginationRequest,
// falling back to defaults when absent or invalid.
func ExtractPage(p *paginationpb.PaginationRequest) (page, pageSize int32) {
	page = DefaultPage
	pageSize = DefaultPageSize
	if p != nil {
		if pm := p.GetPage(); pm != nil {
			if pm.Page > 0 {
				page = pm.Page
			}
			if pm.PageSize > 0 {
				pageSize = pm.PageSize
			}
		}
	}
	return
}

// BuildPageResponse constructs a PaginationResponse for page-based pagination.
func BuildPageResponse(total int64, page, pageSize int32) *paginationpb.PaginationResponse {
	return &paginationpb.PaginationResponse{
		Mode: &paginationpb.PaginationResponse_Page{
			Page: &paginationpb.PagePaginationResponse{
				Total:    total,
				Page:     page,
				PageSize: pageSize,
			},
		},
	}
}
