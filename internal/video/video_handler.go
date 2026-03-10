package video

import (
	"gotik/internal/account"
	"gotik/internal/middleware/jwt"
	"time"

	"github.com/gin-gonic/gin"
)

type VideoHandler struct {
	service        *VideoService
	accountService *account.AccountService
}

func NewVideoHandler(service *VideoService, accountService *account.AccountService) *VideoHandler {
	return &VideoHandler{service: service, accountService: accountService}
}

func (h *VideoHandler) PublishVideo(c *gin.Context) {
	var req PublishVideoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	authorId, err := jwt.GetAccountID(c)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	user, err := h.accountService.FindByID(c.Request.Context(), authorId)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	video := &Video{
		AuthorID:    authorId,
		Username:    user.Username,
		Title:       req.Title,
		Description: req.Description,
		PlayURL:     req.PlayURL,
		CoverURL:    req.CoverURL,
		CreateTime:  time.Now(),
	}
	if err := h.service.Publish(c.Request.Context(), video); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, video)
}

func (h *VideoHandler) ListByAuthorID(c *gin.Context) {
	var req ListByAuthorIDRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	videos, err := h.service.ListByAuthorID(c.Request.Context(), req.AuthorID)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, videos)
}

func (h *VideoHandler) GetDetail(c *gin.Context) {
	var req GetDetailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	video, err := h.service.GetDetail(c.Request.Context(), req.ID)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, video)
}
