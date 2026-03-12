package http

import (
	"gotik/internal/account"
	"gotik/internal/feed"
	jwtmiddleware "gotik/internal/middleware/jwt"
	"gotik/internal/social"
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

	//comment
	commentRepository := video.NewCommentRepository(db)
	commentService := video.NewCommentService(commentRepository, videoRepository)
	commentHandler := video.NewCommentHandler(commentService, accountService)
	commentGroup := r.Group("/comment")
	{
		commentGroup.POST("/listAll", commentHandler.GetAllComments)
	}
	protectedCommentGroup := commentGroup.Group("")
	protectedCommentGroup.Use(jwtmiddleware.JWTAuth(accountRepository))
	{
		protectedCommentGroup.POST("/publish", commentHandler.PublishComment)
		protectedCommentGroup.POST("/delete", commentHandler.DeleteComment)
	}

	//social
	socialRepository := social.NewSocialRepository(db)
	socialService := social.NewSocialService(socialRepository, accountRepository)
	socialHandler := social.NewSocialHandler(socialService)
	socialGroup := r.Group("/social")
	protectedSocialGroup := socialGroup.Group("")
	protectedSocialGroup.Use(jwtmiddleware.JWTAuth(accountRepository))
	{
		protectedSocialGroup.POST("/follow", socialHandler.Follow)
		protectedSocialGroup.POST("/unfollow", socialHandler.Unfollow)
		protectedSocialGroup.POST("/getAllFollowers", socialHandler.GetAllFollowers)
		protectedSocialGroup.POST("/getAllVloggers", socialHandler.GetAllVloggers)
	}

	//feed
	feedRepository := feed.NewFeedRepository(db)
	feedService := feed.NewFeedService(feedRepository, likeRepository)
	feedHandler := feed.NewFeedHandler(feedService)
	feedGroup := r.Group("/feed")
	{
		feedGroup.POST("/listLatest", feedHandler.ListLatest)
		feedGroup.POST("/listLikesCount", feedHandler.ListLikesCount)
	}

	return r
}
