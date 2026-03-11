package http

import (
	"gotik/internal/account"
	jwtmiddleware "gotik/internal/middleware/jwt"
	"gotik/internal/video"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func SetRouter(db *gorm.DB) *gin.Engine {
	r := gin.Default()
	r.Static("/static", "./.run/uploads")

	//account
	accountRepository := account.NewAccountRepository(db)
	accountService := account.NewAccountService(accountRepository)
	accountHandler := account.NewAccountHandler(accountService)

	accountGroup := r.Group("/account")
	{
		accountGroup.POST("/register", accountHandler.CreateAccount)
		accountGroup.POST("/login", accountHandler.Login)
		accountGroup.POST("/findByID", accountHandler.FindByID)
	}

	protectedAccountGroup := accountGroup.Group("")
	protectedAccountGroup.Use(jwtmiddleware.JWTAuth(accountRepository))
	{
		protectedAccountGroup.GET("/me", accountHandler.Me)
	}

	//video
	videoRepository := video.NewVideoRepository(db)
	videoService := video.NewVideoService(videoRepository)
	videoHandler := video.NewVideoHandler(videoService, accountService)
	videoGroup := r.Group("/video")
	{
		videoGroup.POST("/listByAuthorID", videoHandler.ListByAuthorID)
		videoGroup.POST("/getDetail", videoHandler.GetDetail)
	}

	protectedVideoGroup := videoGroup.Group("")
	protectedVideoGroup.Use(jwtmiddleware.JWTAuth(accountRepository))
	{
		protectedVideoGroup.POST("/publish", videoHandler.PublishVideo)
		protectedVideoGroup.POST("/uploadVideo", videoHandler.UploadVideo)
		protectedVideoGroup.POST("/uploadCover", videoHandler.UploadCover)

	}

	//like
	likeRepository := video.NewLikeRepository(db)
	likeService := video.NewLikeService(likeRepository, videoRepository)
	likeHandler := video.NewLikeHandler(likeService)
	likeGroup := r.Group("/like")
	protectedLikeGroup := likeGroup.Group("")
	protectedLikeGroup.Use(jwtmiddleware.JWTAuth(accountRepository))
	{
		protectedLikeGroup.POST("/like", likeHandler.Like)
		protectedLikeGroup.POST("/unlike", likeHandler.Unlike)
		protectedLikeGroup.POST("/isLiked", likeHandler.IsLiked)
		protectedLikeGroup.POST("/listMyLikedVideos", likeHandler.ListMyLikedVideos)
	}
	return r
}
