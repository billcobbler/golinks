package models

import "time"

// Link represents a golink — a short name that redirects to a full URL.
type Link struct {
	ID          int64      `json:"id"`
	Shortname   string     `json:"shortname"`
	TargetURL   string     `json:"target_url"`
	Description string     `json:"description"`
	IsPattern   bool       `json:"is_pattern"`
	CreatedBy   string     `json:"created_by"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	ClickCount  int64      `json:"click_count"`
	LastClicked *time.Time `json:"last_clicked,omitempty"`
}

// ClickEvent records a single redirect event for analytics.
type ClickEvent struct {
	ID        int64     `json:"id"`
	LinkID    int64     `json:"link_id"`
	Shortname string    `json:"shortname,omitempty"`
	ClickedAt time.Time `json:"clicked_at"`
	Referrer  string    `json:"referrer"`
	UserAgent string    `json:"user_agent"`
}

// Stats holds aggregate analytics for the dashboard.
type Stats struct {
	TotalLinks   int64         `json:"total_links"`
	TotalClicks  int64         `json:"total_clicks"`
	TopLinks     []*Link       `json:"top_links"`
	RecentClicks []*ClickEvent `json:"recent_clicks"`
}

// ListResult wraps a paginated list of links with total count.
type ListResult struct {
	Links  []*Link `json:"links"`
	Total  int     `json:"total"`
	Offset int     `json:"offset"`
	Limit  int     `json:"limit"`
}
