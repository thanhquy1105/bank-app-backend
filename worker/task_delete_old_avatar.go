package worker

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog/log"
)

const (
	TaskDeleteOldAvatar = "task:delete_old_avatar"
)

type PayloadDeleteOldAvatar struct {
	Location string `json:"location"`
}

func (distributor *RedisTaskDistributor) DistributeTaskDeleteOldAvatar(
	ctx context.Context,
	payload *PayloadDeleteOldAvatar,
	opts ...asynq.Option,
) error {
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal task payload: %w", err)
	}
	task := asynq.NewTask(TaskDeleteOldAvatar, jsonPayload, opts...)
	info, err := distributor.client.EnqueueContext(ctx, task)
	if err != nil {
		return fmt.Errorf("failed to enqueue task: %w", err)
	}

	log.Info().Str("type", task.Type()).Bytes("payload", task.Payload()).
		Str("queue", info.Queue).Int("max_retry", info.MaxRetry).Msg("enqueue task")

	return nil
}

func (processor *RedisTaskProcessor) ProcessDeleteOldAvatar(ctx context.Context, task *asynq.Task) error {
	var payload PayloadDeleteOldAvatar
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", asynq.SkipRetry)
	}

	err := processor.media.Delete([]string{payload.Location})
	if err != nil {
		return fmt.Errorf("failed to processor task (delete image): %w", err)
	}

	log.Info().Str("type", task.Type()).Bytes("payload", task.Payload()).
		Str("location", payload.Location).Msg("processed task")
	return nil
}
