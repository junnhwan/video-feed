package video

import (
	"net/http"

	"video-feed/internal/account"
	authjwt "video-feed/internal/middleware/jwt"

	"github.com/gin-gonic/gin"
)

type CommentHandler struct {
	service        *CommentService
	accountService *account.Service
}

func NewCommentHandler(service *CommentService, accountService *account.Service) *CommentHandler {
	return &CommentHandler{service: service, accountService: accountService}
}

type publishCommentRequest struct {
	VideoID uint   `json:"video_id"`
	Content string `json:"content"`
}

type deleteCommentRequest struct {
	CommentID uint `json:"comment_id"`
}

type getAllCommentsRequest struct {
	VideoID uint `json:"video_id"`
}

// PublishComment godoc
// @Summary      发表评论
// @Description  通过 MQ 异步落库,失败降级同步写
// @Tags         comment
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body      publishCommentRequest  true  "评论参数"
// @Success      200   {object}  Comment
// @Router       /comment/publish [post]
func (h *CommentHandler) PublishComment(c *gin.Context) {
	var req publishCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	accountID, err := authjwt.GetAccountID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	user, err := h.accountService.FindByID(c.Request.Context(), accountID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	comment, err := h.service.Publish(c.Request.Context(), PublishCommentInput{
		VideoID:  req.VideoID,
		AuthorID: accountID,
		Username: user.Username,
		Content:  req.Content,
	})
	if err != nil {
		c.JSON(statusForInteractionError(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, comment)
}

// DeleteComment godoc
// @Summary      删除评论
// @Tags         comment
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body      deleteCommentRequest  true  "评论 ID"
// @Success      200   {object}  map[string]string
// @Router       /comment/delete [post]
func (h *CommentHandler) DeleteComment(c *gin.Context) {
	var req deleteCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	accountID, err := authjwt.GetAccountID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	if err := h.service.Delete(c.Request.Context(), req.CommentID, accountID); err != nil {
		c.JSON(statusForInteractionError(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "comment deleted successfully"})
}

// GetAllComments godoc
// @Summary      获取评论列表
// @Tags         comment
// @Accept       json
// @Produce      json
// @Param        body  body      getAllCommentsRequest  true  "视频 ID"
// @Success      200   {array}   Comment
// @Router       /comment/getAll [post]
func (h *CommentHandler) GetAllComments(c *gin.Context) {
	var req getAllCommentsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	comments, err := h.service.GetAll(c.Request.Context(), req.VideoID)
	if err != nil {
		c.JSON(statusForInteractionError(err), gin.H{"error": err.Error()})
		return
	}
	if comments == nil {
		comments = []Comment{}
	}
	c.JSON(http.StatusOK, comments)
}
