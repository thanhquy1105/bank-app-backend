package db

import (
	"context"
	"mime/multipart"

	"github.com/jackc/pgx/v5/pgtype"
)

type UpdateAvatarTxParams struct {
	Username     string
	Filename     string
	File         multipart.File
	UploadAvatar func(file multipart.File, filename string) (string, error)
}

type UpdateAvatarTxResult struct {
	Avatar string `json:"avatar_url"`
}

// UpdateAvatarTx performs upload avatar file image to filesys or s3.
func (store *SQLStore) UpdateAvatarTx(ctx context.Context, arg UpdateAvatarTxParams) (UpdateAvatarTxResult, error) {
	var user User

	err := store.execTx(ctx, func(q *Queries) error {
		var err error

		location, err := arg.UploadAvatar(arg.File, arg.Filename)
		if err != nil {
			return err
		}
		params := UpdateUserParams{
			Username: arg.Username,
			Avatar: pgtype.Text{
				String: location,
				Valid:  true,
			},
		}

		user, err = q.UpdateUser(ctx, params)

		return err
	})

	result := UpdateAvatarTxResult{
		Avatar: user.Avatar,
	}

	return result, err
}
