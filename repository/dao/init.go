package dao

import (
	"github.com/asynccnu/be-feed/repository/model"
	"gorm.io/gorm"
)

const FROUNT_NAME = "feed_"

var feedTables = []string{"energy", "grade", "muxi", "holiday"}

func InitTables(db *gorm.DB) error {

	//创建feed消息索引表
	err := db.AutoMigrate(&model.FeedEventIndex{})
	if err != nil {
		return err
	}

	//批量生成多个feed表用来存储不同的feed内容
	for _, table := range feedTables {
		if db.Table(FROUNT_NAME+table).AutoMigrate(&model.FeedEvent{}) != nil {
			return db.Error
		}
	}

	//创建用户配置表
	err = db.AutoMigrate(&model.UserFeedConfig{})
	if err != nil {
		return err
	}

	//创建用户Token表
	err = db.AutoMigrate(&model.Token{})
	if err != nil {
		return err
	}

	//创建用户FeedFailEvent表
	err = db.AutoMigrate(&model.FeedFailEvent{})
	if err != nil {
		return err
	}

	return nil
}
