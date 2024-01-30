package gapi

import (
	"context"
	"errors"
	"fmt"

	db "github.com/thanhquy1105/simplebank/db/sqlc"
	"github.com/thanhquy1105/simplebank/pb"
	"github.com/thanhquy1105/simplebank/util"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (server *Server) Transfer(ctx context.Context, req *pb.TransferRequest) (*pb.TransferResponse, error) {
	authPayload, err := server.authorizeUser(ctx, []string{util.BankerRole, util.DepositorRole})
	if err != nil {
		return nil, unauthenticatedError(err)
	}

	violations := validateTransferRequest(req)
	if violations != nil {
		return nil, invalidArgumentError(violations)
	}

	fromAccount, valid, err := server.validAccount(ctx, req.FromAccountId, req.Currency)
	if !valid {
		return nil, err
	}

	if fromAccount.Owner != authPayload.Username {
		err := errors.New("from account doesn't belong to the authenticated user")
		return nil, status.Errorf(codes.Unauthenticated, "%s", err)
	}

	_, valid, err = server.validAccount(ctx, req.ToAccountId, req.Currency)
	if !valid {
		return nil, err
	}

	arg := db.TransferTxParams{
		FromAccountID: req.FromAccountId,
		ToAccountID:   req.ToAccountId,
		Amount:        req.Amount,
	}

	result, err := server.store.TransferTx(ctx, arg)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%d", err)
	}

	rsp := &pb.TransferResponse{
		Transfer:    convertTransfer(result.Transfer),
		FromAccount: convertAccount(result.FromAccount),
		ToAccount:   convertAccount(result.ToAccount),
		FromEntry:   convertEntry(result.FromEntry),
		ToEntry:     convertEntry(result.ToEntry),
	}
	return rsp, nil

}

func (server *Server) validAccount(ctx context.Context, accountID int64, currency string) (db.Account, bool, error) {
	account, err := server.store.GetAccount(ctx, accountID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return account, false, status.Errorf(codes.NotFound, "%s", err)
		}

		return account, false, status.Errorf(codes.Internal, "%s", err)
	}

	if account.Currency != currency {
		err := fmt.Errorf("account [%d] currency mismatch: %s vs %s", account.ID, account.Currency, currency)
		return account, false, status.Errorf(codes.Internal, "%s", err)
	}
	return account, true, nil
}

func validateTransferRequest(req *pb.TransferRequest) (violations []*errdetails.BadRequest_FieldViolation) {
	if req.GetFromAccountId() < 1 {
		violations = append(violations, fieldViolation("from_account_id", fmt.Errorf("must be greater than 0")))
	}
	if req.GetToAccountId() < 1 {
		violations = append(violations, fieldViolation("to_account_id", fmt.Errorf("must be greater than 0")))
	}
	if req.GetAmount() <= 0 {
		violations = append(violations, fieldViolation("amount", fmt.Errorf("must be greater than 0")))
	}
	if !util.IsSupportedCurrency(req.GetCurrency()) {
		violations = append(violations, fieldViolation("currency", fmt.Errorf("not support this currency")))
	}
	return violations
}
