package gapi

import (
	"context"
	"fmt"

	db "github.com/thanhquy1105/simplebank/db/sqlc"
	"github.com/thanhquy1105/simplebank/pb"
	"github.com/thanhquy1105/simplebank/util"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (server *Server) CreateAccount(ctx context.Context, req *pb.CreateAccountRequest) (*pb.CreateAccountResponse, error) {
	authPayload, err := server.authorizeUser(ctx, []string{util.BankerRole, util.DepositorRole})
	if err != nil {
		return nil, unauthenticatedError(err)
	}

	violations := validateCreateAccountRequest(req)
	if violations != nil {
		return nil, invalidArgumentError(violations)
	}

	arg := db.CreateAccountParams{
		Owner:    authPayload.Username,
		Currency: req.Currency,
		Balance:  0,
	}

	account, err := server.store.CreateAccount(ctx, arg)
	if err != nil {
		errCode := db.ErrorCode(err)
		if errCode == db.UniqueViolation || errCode == db.ForeignKeyViolation {
			return nil, status.Errorf(codes.AlreadyExists, "%s", err)
		}
		return nil, status.Errorf(codes.Internal, "failed to create account: %s", err)
	}

	rsp := &pb.CreateAccountResponse{
		Account: convertAccount(account),
	}
	return rsp, nil

}

func validateCreateAccountRequest(req *pb.CreateAccountRequest) (violations []*errdetails.BadRequest_FieldViolation) {
	if !util.IsSupportedCurrency(req.GetCurrency()) {
		violations = append(violations, fieldViolation("currency", fmt.Errorf("not support this currency")))
	}
	return violations
}
