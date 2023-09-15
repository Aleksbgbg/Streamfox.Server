package models

import (
	"errors"
	"math"
	"time"

	"gorm.io/gorm"
)

type Watch struct {
	UserId Id `gorm:"primaryKey; autoIncrement:false"`
	User   User

	VideoId Id `gorm:"not null"`
	Video   Video

	ViewId Id `gorm:"not null"`

	RowMetadata

	StartedAt     time.Time `gorm:"not null"`
	BytesStreamed *int64    `gorm:"not null"`
}

const WatchPercentageRequired = 0.6

type WatchConditions struct {
	RemainingBytes uint64
	RemainingTime  time.Duration
}

func calculateWatchConditions(watch *Watch, video *Video) WatchConditions {
	minimumBytes := math.Ceil(float64(video.SizeBytes) * WatchPercentageRequired)
	alreadyStreamedBytes := float64(*watch.BytesStreamed)

	minimumWatchTimeMs := math.Ceil(float64(video.DurationSecs) * 1000 * WatchPercentageRequired)
	alreadyWatchedMs := float64(time.Since(watch.StartedAt).Milliseconds())

	return WatchConditions{
		RemainingBytes: uint64(math.Max(0, minimumBytes-alreadyStreamedBytes)),
		RemainingTime: time.Duration(
			math.Max(0, minimumWatchTimeMs-alreadyWatchedMs),
		) * time.Millisecond,
	}
}

func watchFor(user *User, video *Video) (*Watch, error) {
	watch := &Watch{}
	err := db.Where(Watch{UserId: user.Id}).
		Attrs(Watch{
			VideoId:       video.Id,
			ViewId:        NewId(),
			StartedAt:     time.Now(),
			BytesStreamed: new(int64),
		}).
		FirstOrCreate(watch).
		Error

	if err != nil {
		return nil, err
	}

	if watch.VideoId != video.Id {
		watch.VideoId = video.Id
		watch.ViewId = NewId()
		watch.StartedAt = time.Now()
		watch.BytesStreamed = new(int64)
		err = watch.save()

		if err != nil {
			return nil, err
		}
	}

	return watch, nil
}

func watchOrNil(user *User) (*Watch, error) {
	watch := &Watch{}
	err := db.First(watch, user.Id).Error

	if err == nil {
		return watch, nil
	}

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}

	return nil, err
}

func (watch *Watch) save() error {
	return db.Save(watch).Error
}
