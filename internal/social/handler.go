package social

import (
	"gotik/internal/middleware/jwt"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

func NewSocialHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Follow(c *gin.Context) {
	var req FollowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if req.VloggerID == 0 {
		c.JSON(400, gin.H{"error": "vlogger_id is required"})
		return
	}

	followerID, err := jwt.GetAccountID(c)
	if err != nil {
		c.JSON(401, gin.H{"error": err.Error()})
		return
	}

	relation := &Social{
		FollowerID: followerID,
		VloggerID:  req.VloggerID,
	}
	if err := h.service.Follow(c.Request.Context(), relation); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "followed"})
}

func (h *Handler) Unfollow(c *gin.Context) {
	var req UnfollowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if req.VloggerID == 0 {
		c.JSON(400, gin.H{"error": "vlogger_id is required"})
		return
	}

	followerID, err := jwt.GetAccountID(c)
	if err != nil {
		c.JSON(401, gin.H{"error": err.Error()})
		return
	}

	relation := &Social{
		FollowerID: followerID,
		VloggerID:  req.VloggerID,
	}
	if err := h.service.Unfollow(c.Request.Context(), relation); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "unfollowed"})
}

func (h *Handler) GetAllFollowers(c *gin.Context) {
	var req GetAllFollowersRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	vloggerID := req.VloggerID
	if vloggerID == 0 {
		accountID, err := jwt.GetAccountID(c)
		if err != nil {
			c.JSON(401, gin.H{"error": err.Error()})
			return
		}
		vloggerID = accountID
	}

	followers, err := h.service.GetAllFollowers(c.Request.Context(), vloggerID)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, GetAllFollowersResponse{Followers: followers})
}

func (h *Handler) GetAllVloggers(c *gin.Context) {
	var req GetAllVloggersRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	followerID := req.FollowerID
	if followerID == 0 {
		accountID, err := jwt.GetAccountID(c)
		if err != nil {
			c.JSON(401, gin.H{"error": err.Error()})
			return
		}
		followerID = accountID
	}

	vloggers, err := h.service.GetAllVloggers(c.Request.Context(), followerID)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, GetAllVloggersResponse{Vloggers: vloggers})
}
