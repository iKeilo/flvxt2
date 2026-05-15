package repo

import (
	"errors"
	"time"

	"gorm.io/gorm"

	"go-backend/internal/store/model"
)

// UserTrafficHistoryItem 返回给前端的流量历史记录项
type UserTrafficHistoryItem struct {
	ID          int64  `json:"id"`
	UserID      int64  `json:"userId"`
	UserName    string `json:"userName"`
	PeriodKey   int64  `json:"periodKey"`
	InFlow      int64  `json:"inFlow"`
	OutFlow     int64  `json:"outFlow"`
	UsedBytes   int64  `json:"usedBytes"`
	CreatedTime int64  `json:"createdTime"`
}

func (r *Repository) CreateUserTrafficHistory(userID int64, periodKey int64, inFlow int64, outFlow int64) error {
	if r == nil || r.db == nil {
		return errors.New("repository not initialized")
	}
	if userID <= 0 || periodKey <= 0 {
		return errors.New("invalid userID or periodKey")
	}

	var existing model.UserTrafficHistory
	err := r.db.Where("user_id = ? AND period_key = ?", userID, periodKey).First(&existing).Error
	if err == nil {
		return nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	return r.db.Create(&model.UserTrafficHistory{
		UserID:      userID,
		PeriodKey:   periodKey,
		InFlow:      inFlow,
		OutFlow:     outFlow,
		UsedBytes:   inFlow + outFlow,
		CreatedTime: time.Now().UnixMilli(),
	}).Error
}

func (r *Repository) GetUserTrafficHistories(userID int64, limit int) ([]UserTrafficHistoryItem, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("repository not initialized")
	}
	if userID <= 0 {
		return nil, errors.New("invalid userID")
	}
	if limit <= 0 {
		limit = 50
	}

	var items []UserTrafficHistoryItem
	err := r.db.Table("user_traffic_history").
		// COALESCE is compatible with both SQLite and PostgreSQL
		Select("user_traffic_history.id, user_traffic_history.user_id, user_traffic_history.period_key, user_traffic_history.in_flow, user_traffic_history.out_flow, user_traffic_history.used_bytes, user_traffic_history.created_time, COALESCE(user.user, '') as user_name").
		Joins("LEFT JOIN user ON user.id = user_traffic_history.user_id").
		Where("user_traffic_history.user_id = ?", userID).
		Order("user_traffic_history.period_key DESC").
		Limit(limit).
		Scan(&items).Error
	return items, err
}

func (r *Repository) GetAllUserTrafficHistories(limit int) ([]UserTrafficHistoryItem, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("repository not initialized")
	}
	if limit <= 0 {
		limit = 100
	}

	var items []UserTrafficHistoryItem
	err := r.db.Table("user_traffic_history").
		Select("user_traffic_history.id, user_traffic_history.user_id, user_traffic_history.period_key, user_traffic_history.in_flow, user_traffic_history.out_flow, user_traffic_history.used_bytes, user_traffic_history.created_time, COALESCE(user.user, '') as user_name").
		Joins("LEFT JOIN user ON user.id = user_traffic_history.user_id").
		Order("user_traffic_history.created_time DESC").
		Limit(limit).
		Scan(&items).Error
	return items, err
}

func (r *Repository) DeleteUserTrafficHistory(id int64) error {
	if r == nil || r.db == nil {
		return errors.New("repository not initialized")
	}
	if id <= 0 {
		return errors.New("invalid id")
	}
	return r.db.Delete(&model.UserTrafficHistory{}, id).Error
}

// ListUsersForMonthlyReset returns user IDs and their current flows that will be reset this month.
type UserFlowSnapshot struct {
	UserID  int64
	InFlow  int64
	OutFlow int64
}

func (r *Repository) ListUsersForMonthlyReset(day int, lastDay int) ([]UserFlowSnapshot, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("repository not initialized")
	}

	var snapshots []UserFlowSnapshot

	if day == lastDay {
		err := r.db.Raw(
			"SELECT id, in_flow, out_flow FROM user WHERE flow_reset_time != 0 AND (flow_reset_time = ? OR flow_reset_time > ?)",
			day, lastDay,
		).Scan(&snapshots).Error
		return snapshots, err
	}

	err := r.db.Raw(
		"SELECT id, in_flow, out_flow FROM user WHERE flow_reset_time != 0 AND flow_reset_time = ?",
		day,
	).Scan(&snapshots).Error
	return snapshots, err
}
