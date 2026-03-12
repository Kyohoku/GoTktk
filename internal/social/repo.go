package social

import (
	"context"
	"gotik/internal/account"

	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewSocialRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Follow(ctx context.Context, relation *Social) error {
	return r.db.WithContext(ctx).Create(relation).Error
}

func (r *Repository) Unfollow(ctx context.Context, relation *Social) error {
	return r.db.WithContext(ctx).
		Where("follower_id = ? AND vlogger_id = ?", relation.FollowerID, relation.VloggerID).
		Delete(&Social{}).Error
}

func (r *Repository) IsFollowed(ctx context.Context, relation *Social) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&Social{}).
		Where("follower_id = ? AND vlogger_id = ?", relation.FollowerID, relation.VloggerID).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *Repository) GetAllFollowers(ctx context.Context, vloggerID uint) ([]*account.Account, error) {
	var relations []Social
	if err := r.db.WithContext(ctx).
		Where("vlogger_id = ?", vloggerID).
		Find(&relations).Error; err != nil {
		return nil, err
	}

	followerIDs := make([]uint, 0, len(relations))
	for _, relation := range relations {
		followerIDs = append(followerIDs, relation.FollowerID)
	}
	if len(followerIDs) == 0 {
		return []*account.Account{}, nil
	}

	var followers []*account.Account
	if err := r.db.WithContext(ctx).
		Where("id IN ?", followerIDs).
		Find(&followers).Error; err != nil {
		return nil, err
	}
	return followers, nil
}

func (r *Repository) GetAllVloggers(ctx context.Context, followerID uint) ([]*account.Account, error) {
	var relations []Social
	if err := r.db.WithContext(ctx).
		Where("follower_id = ?", followerID).
		Find(&relations).Error; err != nil {
		return nil, err
	}

	vloggerIDs := make([]uint, 0, len(relations))
	for _, relation := range relations {
		vloggerIDs = append(vloggerIDs, relation.VloggerID)
	}
	if len(vloggerIDs) == 0 {
		return []*account.Account{}, nil
	}

	var vloggers []*account.Account
	if err := r.db.WithContext(ctx).
		Where("id IN ?", vloggerIDs).
		Find(&vloggers).Error; err != nil {
		return nil, err
	}
	return vloggers, nil
}
