package video

import (
	"context"
	"errors"
	"regexp"
	"strings"

	rediscache "video-feed/internal/middleware/redis"

	"gorm.io/gorm"
)

var (
	ErrCommentNotFound = errors.New("comment not found")
	ErrUnauthorized    = errors.New("unauthorized")
)

type CommentService struct {
	repo             *CommentRepository
	videoRepo        *Repository
	cache            *rediscache.Client
	commentPublisher CommentEventPublisher
	popPublisher     PopularityEventPublisher
}

type CommentEventPublisher interface {
	Publish(ctx context.Context, username string, videoID uint, authorID uint, content string) error
	Delete(ctx context.Context, commentID uint) error
}

func NewCommentService(repo *CommentRepository, videoRepo *Repository, cache *rediscache.Client) *CommentService {
	return &CommentService{repo: repo, videoRepo: videoRepo, cache: cache}
}

func (s *CommentService) SetPublishers(commentPublisher CommentEventPublisher, popularityPublisher PopularityEventPublisher) {
	s.commentPublisher = commentPublisher
	s.popPublisher = popularityPublisher
}

func (s *CommentService) Publish(ctx context.Context, input PublishCommentInput) (*Comment, error) {
	comment := &Comment{
		VideoID:  input.VideoID,
		AuthorID: input.AuthorID,
		Username: strings.TrimSpace(input.Username),
		Content:  strings.TrimSpace(input.Content),
	}
	if comment.VideoID == 0 || comment.AuthorID == 0 {
		return nil, errors.New("video_id and author_id are required")
	}
	if comment.Content == "" {
		return nil, errors.New("content is required")
	}
	if ok, err := s.videoRepo.IsExist(ctx, comment.VideoID); err != nil {
		return nil, err
	} else if !ok {
		return nil, ErrVideoNotFound
	}

	mysqlEnqueued := false
	redisEnqueued := false
	if s.commentPublisher != nil {
		if err := s.commentPublisher.Publish(ctx, comment.Username, comment.VideoID, comment.AuthorID, comment.Content); err == nil {
			mysqlEnqueued = true
		}
	}
	if s.popPublisher != nil {
		if err := s.popPublisher.Update(ctx, comment.VideoID, 1); err == nil {
			redisEnqueued = true
		}
	}
	if mysqlEnqueued && redisEnqueued {
		s.notifyMentions(ctx, comment)
		return comment, nil
	}

	if !mysqlEnqueued {
		if err := s.applyPublish(ctx, comment); err != nil {
			return nil, err
		}
	}
	if !redisEnqueued {
		UpdatePopularityCache(ctx, s.cache, comment.VideoID, 1)
	}
	s.notifyMentions(ctx, comment)
	return comment, nil
}

func (s *CommentService) Delete(ctx context.Context, commentID uint, accountID uint) error {
	comment, err := s.repo.GetByID(ctx, commentID)
	if err != nil {
		return err
	}
	if comment == nil {
		return ErrCommentNotFound
	}
	if comment.AuthorID != accountID {
		return ErrUnauthorized
	}
	if s.commentPublisher != nil {
		if err := s.commentPublisher.Delete(ctx, commentID); err == nil {
			return nil
		}
	}
	if err := s.applyDelete(ctx, comment); err != nil {
		return err
	}
	UpdatePopularityCache(ctx, s.cache, comment.VideoID, -1)
	return nil
}

func (s *CommentService) applyPublish(ctx context.Context, comment *Comment) error {
	return s.repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(comment).Error; err != nil {
			return err
		}
		return tx.Model(&Video{}).Where("id = ?", comment.VideoID).
			UpdateColumn("popularity", gorm.Expr("popularity + 1")).Error
	})
}

func (s *CommentService) applyDelete(ctx context.Context, comment *Comment) error {
	return s.repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(comment).Error; err != nil {
			return err
		}
		return tx.Model(&Video{}).Where("id = ?", comment.VideoID).
			UpdateColumn("popularity", gorm.Expr("CASE WHEN popularity > 0 THEN popularity - 1 ELSE 0 END")).Error
	})
}

func (s *CommentService) GetAll(ctx context.Context, videoID uint) ([]Comment, error) {
	if videoID == 0 {
		return nil, errors.New("video_id is required")
	}
	if ok, err := s.videoRepo.IsExist(ctx, videoID); err != nil {
		return nil, err
	} else if !ok {
		return nil, ErrVideoNotFound
	}
	return s.repo.GetAllComments(ctx, videoID)
}

var mentionRegex = regexp.MustCompile(`@(\w+)`)

func (s *CommentService) notifyMentions(ctx context.Context, comment *Comment) {
	if s == nil || s.repo == nil || s.repo.db == nil || comment == nil {
		return
	}
	matches := mentionRegex.FindAllStringSubmatch(comment.Content, -1)
	if len(matches) == 0 {
		return
	}
	seen := make(map[string]bool, len(matches))
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		username := strings.TrimSpace(match[1])
		if username == "" || username == comment.Username || seen[username] {
			continue
		}
		seen[username] = true

		var recipientID uint
		err := s.repo.db.WithContext(ctx).
			Table("accounts").
			Where("username = ?", username).
			Select("id").
			Scan(&recipientID).Error
		if err != nil || recipientID == 0 {
			continue
		}
		notification := struct {
			RecipientID uint
			SenderID    uint
			Type        string
			TargetID    uint
			Content     string
		}{
			RecipientID: recipientID,
			SenderID:    comment.AuthorID,
			Type:        "mention",
			TargetID:    comment.VideoID,
			Content:     comment.Username + " 在评论中提到了你",
		}
		_ = s.repo.db.WithContext(ctx).Table("notifications").Create(&notification).Error
	}
}
