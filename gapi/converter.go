package gapi

import (
	db "github.com/thanhquy1105/simplebank/db/sqlc"
	"github.com/thanhquy1105/simplebank/pb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func convertUser(user db.User) *pb.User {
	return &pb.User{
		Username:          user.Username,
		FullName:          user.FullName,
		Email:             user.Email,
		PasswordChangedAt: timestamppb.New(user.PasswordChangedAt),
		CreatedAt:         timestamppb.New(user.CreatedAt),
	}
}

func convertAccount(account db.Account) *pb.Account {
	return &pb.Account{
		Id:        account.ID,
		Owner:     account.Owner,
		Balance:   account.Balance,
		Currency:  account.Currency,
		CreatedAt: timestamppb.New(account.CreatedAt),
	}
}

func convertAccounts(accounts []db.Account) (res []*pb.Account) {
	for _, account := range accounts {
		res = append(res, convertAccount(account))
	}
	return res
}

func convertTransfer(transfer db.Transfer) *pb.Transfer {
	return &pb.Transfer{
		Id:            transfer.ID,
		FromAccountId: transfer.FromAccountID,
		ToAccountId:   transfer.ToAccountID,
		Amount:        transfer.Amount,
		CreatedAt:     timestamppb.New(transfer.CreatedAt),
	}
}

func convertEntry(entry db.Entry) *pb.Entry {
	return &pb.Entry{
		Id:        entry.ID,
		AccountId: entry.AccountID,
		Amount:    entry.Amount,
		CreatedAt: timestamppb.New(entry.CreatedAt),
	}
}
