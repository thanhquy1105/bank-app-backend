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

func (server *Server) AccountList(ctx context.Context, req *pb.AccountListRequest) (*pb.AccountListResponse, error) {
	authPayload, err := server.authorizeUser(ctx, []string{util.BankerRole, util.DepositorRole})
	if err != nil {
		return nil, unauthenticatedError(err)
	}

	violations := validateAccountListRequest(req)
	if violations != nil {
		return nil, invalidArgumentError(violations)
	}

	arg := db.ListAccountsParams{
		Owner:  authPayload.Username,
		Limit:  req.GetPageSize(),
		Offset: (req.GetPageId() - 1) * req.GetPageSize(),
	}
	accounts, err := server.store.ListAccounts(ctx, arg)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get account list:%s", err)
	}

	rsp := &pb.AccountListResponse{
		Accounts: convertAccounts(accounts),
	}
	return rsp, nil

}

func validateAccountListRequest(req *pb.AccountListRequest) (violations []*errdetails.BadRequest_FieldViolation) {
	if req.GetPageId() < 1 {
		violations = append(violations, fieldViolation("page_id", fmt.Errorf("page_id must be greater than 0")))
	}
	if req.GetPageSize() < 5 && req.GetPageSize() > 10 {
		violations = append(violations, fieldViolation("page_size", fmt.Errorf("page_size must be from 5 to 10")))
	}
	return violations
}
