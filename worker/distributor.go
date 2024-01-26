package worker

import (
	"context"

	"github.com/hibiken/asynq"
)

type TaskDistributor interface {
	Close()
	DistributeTaskSendVerifyEmail(
		ctx context.Context,
		payload *PayloadSendVerifyEmail,
		opts ...asynq.Option,
	) error
}

type RedisTaskDistributor struct {
	client *asynq.Client
}

func NewRedisTaskDistributor(redisOpt asynq.RedisClientOpt) TaskDistributor {
	client := asynq.NewClient(redisOpt)
	return &RedisTaskDistributor{
		client: client,
	}
}

func (taskDistributor *RedisTaskDistributor) Close() {
	taskDistributor.client.Close()
}
