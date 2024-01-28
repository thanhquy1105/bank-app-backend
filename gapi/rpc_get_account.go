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

func (server *Server) GetAccount(ctx context.Context, req *pb.GetAccountRequest) (*pb.GetAccountResponse, error) {
	authPayload, err := server.authorizeUser(ctx, []string{util.BankerRole, util.DepositorRole})
	if err != nil {
		return nil, unauthenticatedError(err)
	}

	violations := validateGetAccountRequest(req)
	if violations != nil {
		return nil, invalidArgumentError(violations)
	}

	account, err := server.store.GetAccount(ctx, req.GetId())
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {

			return nil, status.Errorf(codes.NotFound, "%s", err)
		}
		return nil, status.Errorf(codes.Internal, "failed to get account:%s", err)
	}

	if account.Owner != authPayload.Username {
		return nil, status.Errorf(codes.Unauthenticated, "account doesn't belong to the authenticated user")
	}

	rsp := &pb.GetAccountResponse{
		Account: convertAccount(account),
	}
	return rsp, nil

}

func validateGetAccountRequest(req *pb.GetAccountRequest) (violations []*errdetails.BadRequest_FieldViolation) {
	if req.GetId() < 1 {
		violations = append(violations, fieldViolation("id", fmt.Errorf("must be greater than 0")))
	}
	return violations
}
