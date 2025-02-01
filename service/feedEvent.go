package service

import (
	"context"
	"errors"
	"fmt"
	feedv1 "github.com/asynccnu/be-api/gen/proto/feed/v1"
	"github.com/asynccnu/be-feed/domain"
	"github.com/asynccnu/be-feed/events/producer"
	"github.com/asynccnu/be-feed/events/topic"
	"github.com/asynccnu/be-feed/pkg/errorx"
	"github.com/asynccnu/be-feed/pkg/logger"
	"github.com/asynccnu/be-feed/repository/cache"
	"github.com/asynccnu/be-feed/repository/dao"
	"github.com/asynccnu/be-feed/repository/model"
	"sort"
	"sync"
)

// FeedEventService
type FeedEventService interface {
	GetFeedEvents(ctx context.Context, studentId string) (read []domain.FeedEvent, unread []domain.FeedEvent, fail []domain.FeedEvent, err error)
	ReadFeedEvent(ctx context.Context, id int64) error
	ClearFeedEvent(ctx context.Context, studentId string, feedId int64, status string) error
	InsertEventList(ctx context.Context, feedEvents []domain.FeedEvent) []error
	PublicFeedEvent(ctx context.Context, isAll bool, event domain.FeedEvent) error
}

// 定义错误结构体
var (
	GET_FEED_EVENT_ERROR = func(err error) error {
		return errorx.New(feedv1.ErrorGetFeedEventError("获取feed失败"), "dao", err)
	}

	CLEAR_FEED_EVENT_ERROR = func(err error) error {
		return errorx.New(feedv1.ErrorClearFeedEventError("删除feed失败"), "dao", err)
	}
	PUBLIC_FEED_EVENT_ERROR = func(err error) error {
		return errorx.New(feedv1.ErrorPublicFeedEventError("发布feed失败"), "dao", err)
	}
)

type feedEventService struct {
	feedEventDAO      dao.FeedEventDAO
	feedFailEventDAO  dao.FeedFailEventDAO
	feedEventIndexDAO dao.FeedEventIndexDAO
	feedEventCache    cache.FeedEventCache
	userFeedConfigDAO dao.UserFeedConfigDAO
	feedProducer      producer.Producer
	l                 logger.Logger
}

func NewFeedEventService(
	feedEventDAO dao.FeedEventDAO,
	feedEventCache cache.FeedEventCache,
	feedEventIndexDAO dao.FeedEventIndexDAO,
	userFeedConfigDAO dao.UserFeedConfigDAO,
	feedFailEventDAO dao.FeedFailEventDAO,
	feedProducer producer.Producer,
	l logger.Logger,
) FeedEventService {
	return &feedEventService{
		feedEventCache:    feedEventCache,
		feedEventDAO:      feedEventDAO,
		userFeedConfigDAO: userFeedConfigDAO,
		feedEventIndexDAO: feedEventIndexDAO,
		feedFailEventDAO:  feedFailEventDAO,
		feedProducer:      feedProducer,
		l:                 l,
	}
}

// TODO 存储依旧选择按照不同的type分库分表但是查询的时候应该改成原子化的方式,体验应当是无感的(建议进行重构)

// FindPushFeedEvents 根据查询条件查找 Feed 事件
func (s *feedEventService) GetFeedEvents(ctx context.Context, studentId string) (read []domain.FeedEvent, unread []domain.FeedEvent, fail []domain.FeedEvent, err error) {
	events, err := s.feedEventIndexDAO.GetFeedEventIndexListByStudentId(ctx, studentId)
	if err != nil {
		return []domain.FeedEvent{}, []domain.FeedEvent{}, []domain.FeedEvent{}, GET_FEED_EVENT_ERROR(err)
	}

	var (
		mu  sync.Mutex // 用于保护 read 和 unread 的并发访问
		wg  sync.WaitGroup
		sem = make(chan struct{}, 10) // 限制并发数为 10
	)

	for i := range *events {
		wg.Add(1)
		sem <- struct{}{} // 占用一个信号量

		go func(event model.FeedEventIndex) {
			var syncErr error
			defer wg.Done()
			defer func() {
				// 释放信号量
				<-sem
				if syncErr != nil {
					s.l.Error("从数据库获取feedEvent错误", logger.FormatLog("system", syncErr)...)
				}
			}()

			// 直接从数据库中读取
			feedEvent, syncErr := s.feedEventDAO.GetFeedEventById(ctx, event.Type, event.FeedID)
			if syncErr != nil {
				return
			}

			// 根据已读状态分类
			newEvent := domain.FeedEvent{
				ID:           event.ID,
				StudentId:    event.StudentId,
				Type:         event.Type,
				Title:        feedEvent.Title,
				Content:      feedEvent.Content,
				ExtendFields: feedEvent.ExtendFields,
				CreatedAt:    feedEvent.CreatedAt,
			}

			mu.Lock() // 加锁，确保对切片的并发访问是安全的
			if event.Read {
				read = append(read, newEvent)
			} else {
				unread = append(unread, newEvent)
			}
			mu.Unlock()
		}((*events)[i])
	}

	// 等待所有协程完成
	wg.Wait()

	// 根据 ID 排序，从小到大
	sort.Slice(read, func(i, j int) bool {
		return read[i].ID < read[j].ID // 按 ID 升序排列
	})
	sort.Slice(unread, func(i, j int) bool {
		return unread[i].ID < unread[j].ID
	})

	//取出失败消息
	failEvents, err := s.feedFailEventDAO.GetFeedFailEventsByStudentId(ctx, studentId)
	if err != nil {
		return read, unread, []domain.FeedEvent{}, nil
	}

	err = s.feedFailEventDAO.DelFeedFailEventsByStudentId(ctx, studentId)
	if err != nil {
		return read, unread, []domain.FeedEvent{}, nil
	}

	//如果有失败数据则更新
	if len(failEvents) > 0 {
		fail = convFeedFailEventFromModelToDomain(failEvents)
	}

	// 调用 DAO 层的查找方法，返回数据
	return read, unread, fail, nil
}

func (s *feedEventService) ReadFeedEvent(ctx context.Context, id int64) error {
	feedEvent, err := s.feedEventIndexDAO.GetFeedEventIndexById(ctx, id)
	if err != nil {
		return err
	}
	//更新读取状态
	feedEvent.Read = true
	err = s.feedEventIndexDAO.SaveFeedEventIndex(ctx, feedEvent)
	if err != nil {
		return err
	}
	return nil
}

// ClearEvents 清除指定用户的所有 Feed 事件
func (s *feedEventService) ClearFeedEvent(ctx context.Context, studentId string, feedEventId int64, status string) error {
	// 调用 DAO 层的清除方法，删除用户的 Feed 事件
	if feedEventId == 0 && status == "" {
		s.l.Info("意外的清除规则", logger.FormatLog("params", errors.New("参数不足"))...)
		return nil
	}

	err := s.feedEventIndexDAO.RemoveFeedEventIndex(ctx, studentId, feedEventId, status)
	if err != nil {
		return CLEAR_FEED_EVENT_ERROR(err)
	}

	return nil
}

func (s *feedEventService) InsertEventList(ctx context.Context, feedEvents []domain.FeedEvent) []error {
	var errs []error
	// 开始事务，通过 DAO 层进行

	// 分组存储
	groupedEvents := make(map[string][]model.FeedEvent)

	for _, event := range feedEvents {
		tableName := event.Type
		groupedEvents[tableName] = append(groupedEvents[tableName], model.FeedEvent{
			StudentId:    event.StudentId,
			Title:        event.Title,
			Content:      event.Content,
			ExtendFields: event.ExtendFields,
		})
	}
	//记录存储失败的分组
	var errGroups []string
	// 遍历每个组并尝试批量插入对应的表
	for tableName, events := range groupedEvents {
		//排除失败的插入,因为失败的内容之前会插入一次未失败的到索引表
		errGroups = append(errGroups, tableName)
		err := s.insertEventsByType(ctx, tableName, events)
		if err != nil {
			s.l.Error("批量插入feedEvent失败", logger.FormatLog("system", err)...)
			for i := range events {
				err = s.insertEvent(ctx, &events[i], tableName)
				if err != nil {
					s.l.Error("插入feedEvent失败", append(
						logger.FormatLog("system", err),
						logger.String("feedData", fmt.Sprintf("%v", events[i])),
					)...,
					)
					errs = append(errs, err)
				}
			}
		}

	}

	return []error{}
}

func (s *feedEventService) PublicFeedEvent(ctx context.Context, isAll bool, event domain.FeedEvent) error {

	if isAll {

		const batchSize = 50 // 每批次处理的用户数(为什么一次只推送50条呢?主要是怕推送限流有点严重)
		var lastId int64 = 0 // 游标初始值

		for {
			// 获取一批 studentIds
			studentIds, newLastId, err := s.userFeedConfigDAO.GetStudentIdsByCursor(ctx, lastId, batchSize)
			if err != nil {
				s.l.Error("获取用户studentIds错误", append(logger.FormatLog("dao", err), logger.Int64("当前索引:", lastId))...)
			}

			// 如果没有更多数据，结束循环
			if len(studentIds) == 0 {
				return nil
			}

			// 遍历每个学生的 tokens
			for i := range studentIds {
				//更改id并推送
				event.StudentId = studentIds[i]
				err := s.feedProducer.SendMessage(topic.FeedEvent, event)
				if err != nil {
					s.l.Error("发送消息发生失败", append(logger.FormatLog("dao", err), logger.String("当前学号:", studentIds[i]))...)
				}
			}

			// 更新游标为最新值
			lastId = newLastId
		}
	}

	err := s.feedProducer.SendMessage(topic.FeedEvent, event)
	if err != nil {
		return PUBLIC_FEED_EVENT_ERROR(fmt.Errorf("%v,当前学号:%s", err, event.StudentId))
	}
	return nil
}

func (s *feedEventService) insertEvent(ctx context.Context, feedEvent *model.FeedEvent, Type string) error {
	// 开始事务，通过 DAO 层进行
	tx, err := s.feedEventDAO.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()

	//插入feed数据到指定的表
	insertedEvent, err := s.feedEventDAO.InsertFeedEvent(ctx, Type, feedEvent)
	if err != nil {
		return err
	}

	//存储索引到指定的用户表
	err = s.feedEventIndexDAO.SaveFeedEventIndex(ctx, &model.FeedEventIndex{
		StudentId: feedEvent.StudentId,
		FeedID:    insertedEvent.ID,
		Read:      false,
		Type:      Type,
	})
	if err != nil {
		return err
	}

	return nil
}

func (s *feedEventService) insertEventsByType(ctx context.Context, Type string, events []model.FeedEvent) (err error) {
	//开始事务
	tx, err := s.feedEventDAO.BeginTx(ctx)
	if err != nil {
		return err
	}

	//批量存储
	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()

	// 插入 FeedEvent 并获取插入后的 ID
	insertedEvents, err := s.feedEventDAO.InsertFeedEventListByTx(ctx, tx, Type, events)
	if err != nil {
		return err
	}

	var eventIndexes []model.FeedEventIndex
	// 根据插入后的 FeedEvent 生成 FeedEventIndex
	for _, insertedEvent := range insertedEvents {
		eventIndexes = append(eventIndexes, model.FeedEventIndex{
			FeedID:    insertedEvent.ID,
			StudentId: insertedEvent.StudentId,
			Read:      false,
			Type:      Type,
		})
	}

	//批量插入
	err = s.feedEventIndexDAO.InsertFeedEventIndexListByTx(ctx, tx, eventIndexes)
	if err != nil {
		return err
	}

	return nil
}
