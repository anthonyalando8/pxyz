// internal/handler/p2p_admin_grpc_handler.go
package handler

// import (
// 	"context"
// 	"p2p-service/internal/usecase"
// 	//p2ppb "x/shared/genproto/p2p/v1"

// 	"go.uber.org/zap"
// )

// type P2PAdminHandler struct {
// 	p2ppb.UnimplementedP2PAdminServiceServer
// 	profileUsecase *usecase.P2PProfileUsecase
// 	// TODO: Add other usecases
// 	logger *zap.Logger
// }

// func NewP2PAdminHandler(
// 	profileUsecase *usecase.P2PProfileUsecase,
// 	logger *zap.Logger,
// ) *P2PAdminHandler {
// 	return &P2PAdminHandler{
// 		profileUsecase: profileUsecase,
// 		logger:         logger,
// 	}
// }

// // ============================================================================
// // PROFILE MANAGEMENT (Admin)
// // ============================================================================

// // GetProfile retrieves a P2P profile (admin)
// func (h *P2PAdminHandler) GetProfile(ctx context.Context, req *p2ppb.GetProfileRequest) (*p2ppb.GetProfileResponse, error) {
// 	h.logger.Info("Admin getting P2P profile",
// 		zap.Int64("profile_id", req.ProfileId))

// 	// TODO: Implement
// 	return &p2ppb.GetProfileResponse{}, nil
// }

// // ListProfiles lists P2P profiles with filters (admin)
// func (h *P2PAdminHandler) ListProfiles(ctx context.Context, req *p2ppb.ListProfilesRequest) (*p2ppb.ListProfilesResponse, error) {
// 	h.logger.Info("Admin listing P2P profiles")

// 	// TODO: Implement
// 	return &p2ppb.ListProfilesResponse{}, nil
// }

// // SuspendProfile suspends a P2P profile (admin)
// func (h *P2PAdminHandler) SuspendProfile(ctx context.Context, req *p2ppb.SuspendProfileRequest) (*p2ppb.SuspendProfileResponse, error) {
// 	h.logger.Info("Admin suspending profile",
// 		zap.Int64("profile_id", req.ProfileId),
// 		zap.String("reason", req.Reason))

// 	// TODO: Implement
// 	return &p2ppb.SuspendProfileResponse{}, nil
// }

// // UnsuspendProfile unsuspends a P2P profile (admin)
// func (h *P2PAdminHandler) UnsuspendProfile(ctx context.Context, req *p2ppb.UnsuspendProfileRequest) (*p2ppb.UnsuspendProfileResponse, error) {
// 	h.logger.Info("Admin unsuspending profile",
// 		zap.Int64("profile_id", req.ProfileId))

// 	// TODO: Implement
// 	return &p2ppb.UnsuspendProfileResponse{}, nil
// }

// // SetVerified sets verification status (admin)
// func (h *P2PAdminHandler) SetVerified(ctx context.Context, req *p2ppb.SetVerifiedRequest) (*p2ppb.SetVerifiedResponse, error) {
// 	h.logger.Info("Admin setting verification status",
// 		zap.Int64("profile_id", req.ProfileId),
// 		zap.Bool("verified", req.Verified))

// 	// TODO: Implement
// 	return &p2ppb.SetVerifiedResponse{}, nil
// }

// // SetMerchant sets merchant status (admin)
// func (h *P2PAdminHandler) SetMerchant(ctx context.Context, req *p2ppb.SetMerchantRequest) (*p2ppb.SetMerchantResponse, error) {
// 	h.logger.Info("Admin setting merchant status",
// 		zap.Int64("profile_id", req.ProfileId),
// 		zap.Bool("is_merchant", req.IsMerchant))

// 	// TODO: Implement
// 	return &p2ppb.SetMerchantResponse{}, nil
// }

// // ============================================================================
// // AD MANAGEMENT (Admin) - TODO
// // ============================================================================

// // ListAds lists all P2P ads (admin)
// func (h *P2PAdminHandler) ListAds(ctx context.Context, req *p2ppb.ListAdsRequest) (*p2ppb.ListAdsResponse, error) {
// 	// TODO: Implement
// 	return &p2ppb.ListAdsResponse{}, nil
// }

// // SuspendAd suspends an ad (admin)
// func (h *P2PAdminHandler) SuspendAd(ctx context.Context, req *p2ppb.SuspendAdRequest) (*p2ppb.SuspendAdResponse, error) {
// 	// TODO: Implement
// 	return &p2ppb.SuspendAdResponse{}, nil
// }

// // DeleteAd deletes an ad (admin)
// func (h *P2PAdminHandler) DeleteAd(ctx context.Context, req *p2ppb.DeleteAdRequest) (*p2ppb.DeleteAdResponse, error) {
// 	// TODO: Implement
// 	return &p2ppb.DeleteAdResponse{}, nil
// }

// // ============================================================================
// // ORDER MANAGEMENT (Admin) - TODO
// // ============================================================================

// // ListOrders lists all P2P orders (admin)
// func (h *P2PAdminHandler) ListOrders(ctx context.Context, req *p2ppb.ListOrdersRequest) (*p2ppb.ListOrdersResponse, error) {
// 	// TODO: Implement
// 	return &p2ppb.ListOrdersResponse{}, nil
// }

// // GetOrder gets order details (admin)
// func (h *P2PAdminHandler) GetOrder(ctx context.Context, req *p2ppb.GetOrderRequest) (*p2ppb.GetOrderResponse, error) {
// 	// TODO: Implement
// 	return &p2ppb.GetOrderResponse{}, nil
// }

// // CancelOrder cancels an order (admin)
// func (h *P2PAdminHandler) CancelOrder(ctx context.Context, req *p2ppb.CancelOrderRequest) (*p2ppb.CancelOrderResponse, error) {
// 	// TODO: Implement
// 	return &p2ppb.CancelOrderResponse{}, nil
// }

// // ============================================================================
// // DISPUTE MANAGEMENT (Admin) - TODO
// // ============================================================================

// // ListDisputes lists all disputes (admin)
// func (h *P2PAdminHandler) ListDisputes(ctx context.Context, req *p2ppb.ListDisputesRequest) (*p2ppb.ListDisputesResponse, error) {
// 	// TODO: Implement
// 	return &p2ppb.ListDisputesResponse{}, nil
// }

// // GetDispute gets dispute details (admin)
// func (h *P2PAdminHandler) GetDispute(ctx context.Context, req *p2ppb.GetDisputeRequest) (*p2ppb.GetDisputeResponse, error) {
// 	// TODO: Implement
// 	return &p2ppb.GetDisputeResponse{}, nil
// }

// // ResolveDispute resolves a dispute (admin)
// func (h *P2PAdminHandler) ResolveDispute(ctx context.Context, req *p2ppb.ResolveDisputeRequest) (*p2ppb.ResolveDisputeResponse, error) {
// 	// TODO: Implement
// 	return &p2ppb.ResolveDisputeResponse{}, nil
// }

// // ============================================================================
// // REPORT MANAGEMENT (Admin) - TODO
// // ============================================================================

// // ListReports lists all reports (admin)
// func (h *P2PAdminHandler) ListReports(ctx context.Context, req *p2ppb.ListReportsRequest) (*p2ppb.ListReportsResponse, error) {
// 	// TODO: Implement
// 	return &p2ppb.ListReportsResponse{}, nil
// }

// // ReviewReport reviews a report (admin)
// func (h *P2PAdminHandler) ReviewReport(ctx context.Context, req *p2ppb.ReviewReportRequest) (*p2ppb.ReviewReportResponse, error) {
// 	// TODO: Implement
// 	return &p2ppb.ReviewReportResponse{}, nil
// }

// // ============================================================================
// // STATISTICS (Admin) - TODO
// // ============================================================================

// // GetP2PStats gets overall P2P statistics (admin)
// func (h *P2PAdminHandler) GetP2PStats(ctx context.Context, req *p2ppb.GetP2PStatsRequest) (*p2ppb.GetP2PStatsResponse, error) {
// 	// TODO: Implement
// 	return &p2ppb.GetP2PStatsResponse{}, nil
// }