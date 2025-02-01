// Code generated by Wire. DO NOT EDIT.

//go:generate go run -mod=mod github.com/google/wire/cmd/wire
//go:build !wireinject
// +build !wireinject

package main

import (
	"github.com/asynccnu/be-feed/cron"
	"github.com/asynccnu/be-feed/events"
	"github.com/asynccnu/be-feed/events/producer"
	"github.com/asynccnu/be-feed/grpc"
	"github.com/asynccnu/be-feed/ioc"
	"github.com/asynccnu/be-feed/repository/cache"
	"github.com/asynccnu/be-feed/repository/dao"
	"github.com/asynccnu/be-feed/service"
)

// Injectors from wire.go:

func InitApp() App {
	logger := ioc.InitLogger()
	db := ioc.InitDB(logger)
	feedEventDAO := dao.NewFeedEventDAO(db)
	cmdable := ioc.InitRedis()
	feedEventCache := cache.NewRedisFeedEventCache(cmdable)
	feedEventIndexDAO := dao.NewFeedEventIndexDAO(db)
	userFeedConfigDAO := dao.NewUserFeedConfigDAO(db)
	feedFailEventDAO := dao.NewFeedFailEventDAO(db)
	client := ioc.InitKafka()
	producerProducer := producer.NewSaramaProducer(client)
	feedEventService := service.NewFeedEventService(feedEventDAO, feedEventCache, feedEventIndexDAO, userFeedConfigDAO, feedFailEventDAO, producerProducer, logger)
	userFeedTokenDAO := dao.NewUserFeedTokenDAO(db)
	feedUserConfigService := service.NewFeedUserConfigService(feedEventDAO, feedEventCache, userFeedConfigDAO, userFeedTokenDAO)
	muxiOfficialMSGService := service.NewMuxiOfficialMSGService(feedEventDAO, feedEventCache, userFeedConfigDAO)
	pushClient := ioc.InitJPushClient()
	pushService := service.NewPushService(pushClient, userFeedConfigDAO, userFeedTokenDAO, feedFailEventDAO, logger)
	feedServiceServer := grpc.NewFeedServiceServer(feedEventService, feedUserConfigService, muxiOfficialMSGService, pushService, logger)
	clientv3Client := ioc.InitEtcdClient()
	server := ioc.InitGRPCxKratosServer(feedServiceServer, clientv3Client, logger)
	muxiController := cron.NewMuxiController(muxiOfficialMSGService, feedEventService, pushService, logger)
	v := cron.NewCron(muxiController)
	feedEventConsumerHandler := events.NewFeedEventConsumerHandler(client, logger, feedEventService, pushService)
	v2 := ioc.InitConsumers(feedEventConsumerHandler)
	app := NewApp(server, v, v2)
	return app
}
