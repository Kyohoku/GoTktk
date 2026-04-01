package ai

//ai相关的请求、响应、上下文以及 Python 子服务的 DTO

type GenerateSummaryRequest struct {
	VideoID uint `json:"video_id" binding:"required"`
}

type GetSummaryRequest struct {
	VideoID uint `json:"video_id" binding:"required"`
}

type GenerateCommentSuggestionsRequest struct {
	VideoID uint   `json:"video_id" binding:"required"`
	Style   string `json:"style"`
}

type GetCommentSuggestionsRequest struct {
	VideoID uint   `json:"video_id" binding:"required"`
	Style   string `json:"style"`
}

type QARequest struct {
	VideoID  uint   `json:"video_id" binding:"required"`
	Question string `json:"question" binding:"required"`
}

type VideoContext struct {
	VideoID        uint
	AuthorID       uint
	AuthorName     string
	Title          string
	Description    string
	Tags           []string
	Transcript     string
	CommentSummary string
	SourceText     string
	SourceTextHash string
}

type SummaryResult struct {
	Summary         string   `json:"summary"`
	Keywords        []string `json:"keywords"`
	Audience        string   `json:"audience"`
	RecommendReason string   `json:"recommend_reason"`
}

type CommentSuggestionsResult struct {
	Style       string   `json:"style"`
	Suggestions []string `json:"suggestions"`
}

type QAResult struct {
	Answer string `json:"answer"`
}

type PythonSummaryRequest struct {
	Context VideoContext `json:"context"`
}

type PythonCommentSuggestionsRequest struct {
	Context VideoContext `json:"context"`
	Style   string       `json:"style"`
}

type PythonQARequest struct {
	Context  VideoContext `json:"context"`
	Question string       `json:"question"`
}
