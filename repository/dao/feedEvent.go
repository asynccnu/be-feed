package dao

import (
	"context"
	"github.com/asynccnu/be-feed/repository/model"
	"gorm.io/gorm"
	"time"
)

// feedEvent由于使用了表名进行查询,gorm的自动处理时间的作用将失效
type FeedEventDAO interface {
	// 上部分是用于对 index 进行处理,下部分是对具体的 feedEvent 进行处理

	GetFeedEventById(ctx context.Context, feedType string, Id int64) (*model.FeedEvent, error)
	GetFeedEventsByStudentId(ctx context.Context, feedType string, studentId string) ([]model.FeedEvent, error)
	DelFeedEventById(ctx context.Context, feedType string, Id int64) error
	DelFeedEventsByStudentId(ctx context.Context, feedType string, studentId string) error
	InsertFeedEventList(ctx context.Context, feedType string, event []model.FeedEvent) ([]model.FeedEvent, error)
	InsertFeedEvent(ctx context.Context, feedType string, event *model.FeedEvent) (*model.FeedEvent, error)
	InsertFeedEventListByTx(ctx context.Context, tx *gorm.DB, feedType string, events []model.FeedEvent) ([]model.FeedEvent, error)
	// 用于事务
	BeginTx(ctx context.Context) (*gorm.DB, error)
}

type feedEventDAO struct {
	gorm *gorm.DB
}

func NewFeedEventDAO(db *gorm.DB) FeedEventDAO {
	return &feedEventDAO{gorm: db}
}

// GetFeedEventById 获取指定 ID 的 FeedEvent
func (dao *feedEventDAO) GetFeedEventById(ctx context.Context, feedType string, Id int64) (*model.FeedEvent, error) {
	d := model.FeedEvent{}
	err := dao.gorm.WithContext(ctx).Table(FROUNT_NAME+feedType).
		Where("id = ?", Id). // 过滤已软删除的记录
		First(&d).Error
	return &d, err
}

// GetFeedEventsByStudentId 获取指定 StudentId 的 FeedEvent 列表
func (dao *feedEventDAO) GetFeedEventsByStudentId(ctx context.Context, feedType string, studentId string) ([]model.FeedEvent, error) {
	var resp []model.FeedEvent
	err := dao.gorm.WithContext(ctx).Table(FROUNT_NAME+feedType).
		Where("student_id = ?", studentId). // 过滤已软删除的记录
		Find(&resp).Error
	return resp, err
}

// DelFeedEventById 软删除指定 ID 的 FeedEvent
func (dao *feedEventDAO) DelFeedEventById(ctx context.Context, feedType string, Id int64) error {
	return dao.gorm.WithContext(ctx).Table(FROUNT_NAME+feedType).
		Where("id = ?", Id).
		Update("deleted_at", time.Now().Unix()). // 设置软删除时间
		Error
}

// DelFeedEventsByStudentId 软删除指定 StudentId 的 FeedEvent
func (dao *feedEventDAO) DelFeedEventsByStudentId(ctx context.Context, feedType string, studentId string) error {
	return dao.gorm.WithContext(ctx).Table(FROUNT_NAME+feedType).
		Where("student_id = ?", studentId).
		Update("deleted_at", time.Now().Unix()). // 设置软删除时间
		Error
}

// InsertFeedEventList 批量插入 FeedEvent，最多一次插入 1000 条
func (dao *feedEventDAO) InsertFeedEventList(ctx context.Context, feedType string, events []model.FeedEvent) ([]model.FeedEvent, error) {
	now := time.Now().Unix()
	// 为每个事件设置时间戳
	for i := range events {
		events[i].CreatedAt = now
		events[i].UpdatedAt = now
	}
	err := dao.gorm.WithContext(ctx).Table(FROUNT_NAME+feedType).CreateInBatches(events, 1000).Error
	return events, err
}

// InsertFeedEvent 插入单个 FeedEvent
func (dao *feedEventDAO) InsertFeedEvent(ctx context.Context, feedType string, event *model.FeedEvent) (*model.FeedEvent, error) {
	now := time.Now().Unix()
	event.CreatedAt = now
	event.UpdatedAt = now
	err := dao.gorm.WithContext(ctx).Table(FROUNT_NAME + feedType).Create(event).Error
	return event, err
}

// InsertFeedEventListByTx 使用事务批量插入 FeedEvent，最多一次插入 1000 条
func (dao *feedEventDAO) InsertFeedEventListByTx(ctx context.Context, tx *gorm.DB, feedType string, events []model.FeedEvent) ([]model.FeedEvent, error) {
	now := time.Now().Unix()
	// 为每个事件设置时间戳
	for i := range events {
		events[i].CreatedAt = now
		events[i].UpdatedAt = now
	}
	err := tx.WithContext(ctx).Table(FROUNT_NAME+feedType).CreateInBatches(events, 1000).Error
	return events, err
}

// BeginTx 开启事务，用于批量插入或其他操作
func (dao *feedEventDAO) BeginTx(ctx context.Context) (*gorm.DB, error) {
	tx := dao.gorm.WithContext(ctx).Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}
	return tx, nil
}
