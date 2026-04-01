package ai

import "time"

type VideoSummary struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	VideoID         uint      `gorm:"uniqueIndex;not null" json:"video_id"`
	Summary         string    `gorm:"type:text;not null" json:"summary"`
	Keywords        string    `gorm:"type:json" json:"keywords"`
	Audience        string    `gorm:"type:varchar(255)" json:"audience"`
	RecommendReason string    `gorm:"type:text" json:"recommend_reason"`
	SourceTextHash  string    `gorm:"type:varchar(64)" json:"source_text_hash"`
	Status          string    `gorm:"type:varchar(32);not null;default:done" json:"status"`
	ErrorMessage    string    `gorm:"type:varchar(255)" json:"error_message"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func (VideoSummary) TableName() string {
	return "ai_video_summaries"
}

type CommentSuggestion struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	VideoID        uint      `gorm:"uniqueIndex:idx_video_style;not null" json:"video_id"`
	Style          string    `gorm:"uniqueIndex:idx_video_style;type:varchar(32);not null;default:default" json:"style"`
	Suggestions    string    `gorm:"type:json;not null" json:"suggestions"`
	SourceTextHash string    `gorm:"type:varchar(64)" json:"source_text_hash"`
	Status         string    `gorm:"type:varchar(32);not null;default:done" json:"status"`
	ErrorMessage   string    `gorm:"type:varchar(255)" json:"error_message"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (CommentSuggestion) TableName() string {
	return "ai_comment_suggestions"
}

type QAHistory struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	VideoID       uint      `gorm:"index;not null" json:"video_id"`
	AccountID     uint      `gorm:"index" json:"account_id"`
	Question      string    `gorm:"type:text;not null" json:"question"`
	Answer        string    `gorm:"type:text;not null" json:"answer"`
	ContextSource string    `gorm:"type:varchar(32);default:video_context" json:"context_source"`
	CreatedAt     time.Time `json:"created_at"`
}

func (QAHistory) TableName() string {
	return "ai_qa_histories"
}
