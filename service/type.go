package service

import (
	"github.com/asynccnu/be-feed/domain"
	"github.com/asynccnu/be-feed/repository/cache"
	"github.com/asynccnu/be-feed/repository/model"
)

func convFeedEventFromModelToDomain(feedEvents []model.FeedEvent, Type string) []domain.FeedEvent {
	result := make([]domain.FeedEvent, len(feedEvents)) // 直接预分配
	for i := range feedEvents {
		result[i] = domain.FeedEvent{ // 通过索引直接赋值
			ID:           feedEvents[i].ID,
			StudentId:    feedEvents[i].StudentId,
			Type:         Type,
			Title:        feedEvents[i].Title,
			Content:      feedEvents[i].Content,
			ExtendFields: feedEvents[i].ExtendFields,
			CreatedAt:    feedEvents[i].CreatedAt,
		}
	}
	return result
}

func convFeedFailEventFromModelToDomain(feedEvents []model.FeedFailEvent) []domain.FeedEvent {
	result := make([]domain.FeedEvent, len(feedEvents)) // 直接预分配
	for i := range feedEvents {
		result[i] = domain.FeedEvent{ // 通过索引直接赋值
			ID:           feedEvents[i].ID,
			StudentId:    feedEvents[i].StudentId,
			Type:         feedEvents[i].Type,
			Title:        feedEvents[i].Title,
			Content:      feedEvents[i].Content,
			ExtendFields: feedEvents[i].ExtendFields,
			CreatedAt:    feedEvents[i].CreatedAt,
		}
	}
	return result
}
func convFeedFailEventFromDomainToModel(feedEvents []domain.FeedEvent) []model.FeedFailEvent {
	result := make([]model.FeedFailEvent, len(feedEvents)) // 直接预分配
	for i := range feedEvents {
		result[i] = model.FeedFailEvent{
			StudentId:    feedEvents[i].StudentId,
			Type:         feedEvents[i].Type,
			Title:        feedEvents[i].Title,
			Content:      feedEvents[i].Content,
			ExtendFields: feedEvents[i].ExtendFields,
		}
	}
	return result
}

func convMuxiMessageFromCacheToDomain(feeds []cache.MuxiOfficialMSG) []domain.MuxiOfficialMSG {
	//类型转换
	result := make([]domain.MuxiOfficialMSG, len(feeds))
	for i := range feeds {
		result[i] = domain.MuxiOfficialMSG{
			Id:           feeds[i].MuixMSGId,
			Title:        feeds[i].Title,
			Content:      feeds[i].Content,
			ExtendFields: domain.ExtendFields(feeds[i].ExtendFields),
			PublicTime:   feeds[i].PublicTime,
		}
	}
	return result
}
