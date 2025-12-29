package models

// Pagination holds pagination parameters.
type Pagination struct {
	Page     int
	PageSize int
}

// DefaultPagination returns default pagination settings.
func DefaultPagination() Pagination {
	return Pagination{
		Page:     1,
		PageSize: 25,
	}
}

// Offset calculates the SQL offset for the current page.
func (p Pagination) Offset() int {
	if p.Page < 1 {
		p.Page = 1
	}
	return (p.Page - 1) * p.PageSize
}

// Limit returns the page size as limit.
func (p Pagination) Limit() int {
	if p.PageSize < 1 {
		return 25
	}
	if p.PageSize > 100 {
		return 100
	}
	return p.PageSize
}

// TotalPages calculates the total number of pages.
func (p Pagination) TotalPages(total int) int {
	if p.PageSize <= 0 {
		return 1
	}
	pages := total / p.PageSize
	if total%p.PageSize > 0 {
		pages++
	}
	if pages < 1 {
		return 1
	}
	return pages
}

// SortDirection represents the sort direction.
type SortDirection string

const (
	SortAsc  SortDirection = "ASC"
	SortDesc SortDirection = "DESC"
)

// SortOption defines a column sort option.
type SortOption struct {
	Column    string
	Direction SortDirection
}
