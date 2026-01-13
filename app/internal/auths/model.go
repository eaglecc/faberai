package auths

import (
	"context"
	"model"

	"github.com/google/uuid"
	"github.com/mszlu521/thunder/gorms"
	"gorm.io/gorm"
)

type Models struct {
	db *gorm.DB
}

func NewModel(db *gorm.DB) *Models {
	return &Models{db: db}
}

func (m *Models) findByUsername(ctx context.Context, username string) (*model.User, error) {
	var user model.User
	err := m.db.WithContext(ctx).Where("username = ?", username).First(&user).Error
	if gorms.IsRecordNotFoundError(err) {
		return nil, nil
	}
	return &user, err
}

func (m *Models) findByEmail(ctx context.Context, email string) (*model.User, error) {
	var user model.User
	err := m.db.WithContext(ctx).Where("email = ?", email).First(&user).Error
	if gorms.IsRecordNotFoundError(err) {
		return nil, nil
	}
	return &user, err
}

func (m *Models) transaction(ctx context.Context, f func(tx *gorm.DB) error) error {
	return m.db.WithContext(ctx).Transaction(f)
}

func (m *Models) saveUser(ctx context.Context, tx *gorm.DB, user *model.User) error {
	if tx == nil {
		tx = m.db
	}
	return tx.WithContext(ctx).Create(user).Error
}

func (m *Models) findById(ctx context.Context, id uuid.UUID) (*model.User, error) {
	var user model.User
	err := m.db.WithContext(ctx).Where("id = ?", id).First(&user).Error
	if gorms.IsRecordNotFoundError(err) {
		return nil, nil
	}
	return &user, err
}

func (m *Models) updateUser(ctx context.Context, tx *gorm.DB, u *model.User) error {
	if tx == nil {
		tx = m.db
	}
	return tx.WithContext(ctx).Updates(u).Error
}

func (m *Models) findByUsernameOrEmail(ctx context.Context, username string) (*model.User, error) {
	var user model.User
	err := m.db.WithContext(ctx).Where("username = ? OR email = ?", username, username).First(&user).Error
	if gorms.IsRecordNotFoundError(err) {
		return nil, nil
	}
	return &user, err
}
