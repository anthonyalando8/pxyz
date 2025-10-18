package handler

import (
	"audit-service/internal/domain"
	//"time"

	emailclient "x/shared/email"
	smsclient "x/shared/sms"
	notificationclient "x/shared/notification"

	auditpb "x/shared/genproto/authentication/audit-service/grpcpb"

	"context"
	// "fmt"
	// "log"

	"audit-service/internal/service/audit"

	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// security_audit_grpc_server.go

type SecurityAuditGRPCServer struct {
	auditpb.UnimplementedSecurityAuditServiceServer
	auditService *service.SecurityAuditService

	emailClient *emailclient.EmailClient
	smsClient *smsclient.SMSClient
	notificationClient *notificationclient.NotificationService
}

func NewSecurityAuditGRPCServer(
	auditService *service.SecurityAuditService,
	emailClient *emailclient.EmailClient,
	smsClient *smsclient.SMSClient,
	notificationClient *notificationclient.NotificationService,
	) *SecurityAuditGRPCServer {
	return &SecurityAuditGRPCServer{
		auditService: auditService,
		emailClient:        emailClient,
		smsClient:          smsClient,
		notificationClient: notificationClient, 
	}
}

// ================================
// ACCOUNT LOCK OPERATIONS
// ================================

func (s *SecurityAuditGRPCServer) CheckAccountLockStatus(ctx context.Context, req *auditpb.CheckAccountLockStatusRequest) (*auditpb.CheckAccountLockStatusResponse, error) {
	lockout, err := s.auditService.CheckAccountLockStatus(ctx, req.UserId)
	if err != nil {
		return nil, err
	}

	if lockout == nil {
		return &auditpb.CheckAccountLockStatusResponse{
			IsLocked: false,
		}, nil
	}

	return &auditpb.CheckAccountLockStatusResponse{
		IsLocked: true,
		Lockout:  toProtoAccountLockout(lockout),
	}, nil
}

func (s *SecurityAuditGRPCServer) CheckAccountsLockStatus(ctx context.Context, req *auditpb.CheckAccountsLockStatusRequest) (*auditpb.CheckAccountsLockStatusResponse, error) {
	statuses := make([]*auditpb.AccountLockStatus, 0, len(req.UserIds))

	for _, userID := range req.UserIds {
		lockout, err := s.auditService.CheckAccountLockStatus(ctx, userID)
		if err != nil {
			// Log error but continue with other users
			statuses = append(statuses, &auditpb.AccountLockStatus{
				UserId:   userID,
				IsLocked: false,
			})
			continue
		}

		status := &auditpb.AccountLockStatus{
			UserId:   userID,
			IsLocked: lockout != nil,
		}

		if lockout != nil {
			status.Lockout = toProtoAccountLockout(lockout)
		}

		statuses = append(statuses, status)
	}

	return &auditpb.CheckAccountsLockStatusResponse{
		Statuses: statuses,
	}, nil
}

func (s *SecurityAuditGRPCServer) UnlockAccount(ctx context.Context, req *auditpb.UnlockAccountRequest) (*auditpb.UnlockAccountResponse, error) {
	auditCtx := &service.AuditContext{
		IPAddress: &req.IpAddress,
	}

	err := s.auditService.UnlockAccount(ctx, req.UserId, req.UnlockedBy, auditCtx)
	if err != nil {
		return &auditpb.UnlockAccountResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	return &auditpb.UnlockAccountResponse{
		Success: true,
		Message: "Account unlocked successfully",
	}, nil
}

func (s *SecurityAuditGRPCServer) UnlockAccounts(ctx context.Context, req *auditpb.UnlockAccountsRequest) (*auditpb.UnlockAccountsResponse, error) {
	results := make([]*auditpb.UnlockResult, 0, len(req.UserIds))
	unlockedCount := int32(0)

	auditCtx := &service.AuditContext{
		IPAddress: &req.IpAddress,
	}

	for _, userID := range req.UserIds {
		err := s.auditService.UnlockAccount(ctx, userID, req.UnlockedBy, auditCtx)
		
		result := &auditpb.UnlockResult{
			UserId:  userID,
			Success: err == nil,
		}

		if err != nil {
			result.Error = err.Error()
		} else {
			unlockedCount++
		}

		results = append(results, result)
	}

	return &auditpb.UnlockAccountsResponse{
		UnlockedCount: unlockedCount,
		Results:       results,
	}, nil
}

// ================================
// SUSPICIOUS ACTIVITY OPERATIONS
// ================================

func (s *SecurityAuditGRPCServer) DetectSuspiciousActivity(ctx context.Context, req *auditpb.DetectSuspiciousActivityRequest) (*auditpb.DetectSuspiciousActivityResponse, error) {
	auditCtx := &service.AuditContext{
		IPAddress: &req.IpAddress,
		SessionID: &req.SessionId,
		UserAgent: &req.UserAgent,
	}

	err := s.auditService.DetectSuspiciousActivity(ctx, req.UserId, req.IpAddress, auditCtx)
	if err != nil {
		return nil, err
	}

	// Get risk score
	riskScore, _ := s.auditService.GetUserRiskScore(ctx, req.UserId)

	// Get active suspicious activities
	activities, _ := s.auditService.GetActiveSuspiciousActivities(ctx, req.UserId)

	activityTypes := make([]string, 0)
	for _, activity := range activities {
		activityTypes = append(activityTypes, activity.ActivityType)
	}

	return &auditpb.DetectSuspiciousActivityResponse{
		SuspiciousDetected: len(activities) > 0,
		RiskScore:          int32(riskScore),
		Activities:         activityTypes,
	}, nil
}

func (s *SecurityAuditGRPCServer) ReportSuspiciousActivity(ctx context.Context, req *auditpb.ReportSuspiciousActivityRequest) (*auditpb.ReportSuspiciousActivityResponse, error) {
	details := req.Details.AsMap()

	reportReq := &domain.ReportSuspiciousActivityRequest{
		UserID:       req.UserId,
		ActivityType: req.ActivityType,
		RiskScore:    int(req.RiskScore),
		IPAddress:    &req.IpAddress,
		Details:      details,
	}

	err := s.auditService.ReportSuspiciousActivity(ctx, reportReq, req.ReportedBy)
	if err != nil {
		return &auditpb.ReportSuspiciousActivityResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	return &auditpb.ReportSuspiciousActivityResponse{
		Success: true,
		Message: "Suspicious activity reported",
	}, nil
}

func (s *SecurityAuditGRPCServer) ReportSuspiciousActivities(ctx context.Context, req *auditpb.ReportSuspiciousActivitiesRequest) (*auditpb.ReportSuspiciousActivitiesResponse, error) {
	results := make([]*auditpb.ReportResult, 0, len(req.Activities))
	reportedCount := int32(0)

	for _, activity := range req.Activities {
		details := activity.Details.AsMap()

		reportReq := &domain.ReportSuspiciousActivityRequest{
			UserID:       activity.UserId,
			ActivityType: activity.ActivityType,
			RiskScore:    int(activity.RiskScore),
			IPAddress:    &activity.IpAddress,
			Details:      details,
		}

		err := s.auditService.ReportSuspiciousActivity(ctx, reportReq, activity.ReportedBy)

		result := &auditpb.ReportResult{
			UserId:  activity.UserId,
			Success: err == nil,
		}

		if err != nil {
			result.Error = err.Error()
		} else {
			reportedCount++
		}

		results = append(results, result)
	}

	return &auditpb.ReportSuspiciousActivitiesResponse{
		ReportedCount: reportedCount,
		Results:       results,
	}, nil
}

func (s *SecurityAuditGRPCServer) ResolveSuspiciousActivity(ctx context.Context, req *auditpb.ResolveSuspiciousActivityRequest) (*auditpb.ResolveSuspiciousActivityResponse, error) {
	err := s.auditService.ResolveSuspiciousActivity(ctx, req.ActivityId, req.ResolvedBy, req.Status)
	if err != nil {
		return &auditpb.ResolveSuspiciousActivityResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	return &auditpb.ResolveSuspiciousActivityResponse{
		Success: true,
		Message: "Suspicious activity resolved",
	}, nil
}

func (s *SecurityAuditGRPCServer) ResolveSuspiciousActivities(ctx context.Context, req *auditpb.ResolveSuspiciousActivitiesRequest) (*auditpb.ResolveSuspiciousActivitiesResponse, error) {
	results := make([]*auditpb.ResolveResult, 0, len(req.ActivityIds))
	resolvedCount := int32(0)

	for _, activityID := range req.ActivityIds {
		err := s.auditService.ResolveSuspiciousActivity(ctx, activityID, req.ResolvedBy, req.Status)

		result := &auditpb.ResolveResult{
			ActivityId: activityID,
			Success:    err == nil,
		}

		if err != nil {
			result.Error = err.Error()
		} else {
			resolvedCount++
		}

		results = append(results, result)
	}

	return &auditpb.ResolveSuspiciousActivitiesResponse{
		ResolvedCount: resolvedCount,
		Results:       results,
	}, nil
}

func (s *SecurityAuditGRPCServer) GetActiveSuspiciousActivities(ctx context.Context, req *auditpb.GetActiveSuspiciousActivitiesRequest) (*auditpb.GetActiveSuspiciousActivitiesResponse, error) {
	activities, err := s.auditService.GetActiveSuspiciousActivities(ctx, req.UserId)
	if err != nil {
		return nil, err
	}

	protoActivities := make([]*auditpb.SuspiciousActivity, 0, len(activities))
	for _, activity := range activities {
		protoActivities = append(protoActivities, toProtoSuspiciousActivity(activity))
	}

	return &auditpb.GetActiveSuspiciousActivitiesResponse{
		Activities: protoActivities,
	}, nil
}

func (s *SecurityAuditGRPCServer) GetHighRiskUsers(ctx context.Context, req *auditpb.GetHighRiskUsersRequest) (*auditpb.GetHighRiskUsersResponse, error) {
	activities, err := s.auditService.GetHighRiskUsers(ctx, int(req.MinRiskScore), int(req.Limit))
	if err != nil {
		return nil, err
	}

	protoActivities := make([]*auditpb.SuspiciousActivity, 0, len(activities))
	for _, activity := range activities {
		protoActivities = append(protoActivities, toProtoSuspiciousActivity(activity))
	}

	return &auditpb.GetHighRiskUsersResponse{
		Activities: protoActivities,
	}, nil
}

// ================================
// AUDIT LOG QUERIES
// ================================

func (s *SecurityAuditGRPCServer) GetUserAuditHistory(ctx context.Context, req *auditpb.GetUserAuditHistoryRequest) (*auditpb.GetUserAuditHistoryResponse, error) {
	limit := int(req.Limit)
	if limit <= 0 {
		limit = 100
	}

	logs, err := s.auditService.GetUserAuditHistory(ctx, req.UserId, limit)
	if err != nil {
		return nil, err
	}

	protoLogs := make([]*auditpb.SecurityAuditLog, 0, len(logs))
	for _, log := range logs {
		protoLogs = append(protoLogs, toProtoSecurityAuditLog(log))
	}

	return &auditpb.GetUserAuditHistoryResponse{
		Logs: protoLogs,
	}, nil
}

func (s *SecurityAuditGRPCServer) GetUsersAuditHistory(ctx context.Context, req *auditpb.GetUsersAuditHistoryRequest) (*auditpb.GetUsersAuditHistoryResponse, error) {
	limit := int(req.Limit)
	if limit <= 0 {
		limit = 100
	}

	histories := make([]*auditpb.UserAuditHistory, 0, len(req.UserIds))

	for _, userID := range req.UserIds {
		logs, err := s.auditService.GetUserAuditHistory(ctx, userID, limit)
		if err != nil {
			// Log error but continue
			continue
		}

		protoLogs := make([]*auditpb.SecurityAuditLog, 0, len(logs))
		for _, log := range logs {
			protoLogs = append(protoLogs, toProtoSecurityAuditLog(log))
		}

		histories = append(histories, &auditpb.UserAuditHistory{
			UserId: userID,
			Logs:   protoLogs,
		})
	}

	return &auditpb.GetUsersAuditHistoryResponse{
		Histories: histories,
	}, nil
}

func (s *SecurityAuditGRPCServer) QueryAuditLogs(ctx context.Context, req *auditpb.QueryAuditLogsRequest) (*auditpb.QueryAuditLogsResponse, error) {
	query := &domain.AuditLogQuery{
		Limit:  int(req.Limit),
		Offset: int(req.Offset),
	}

	if req.UserId != "" {
		query.UserID = &req.UserId
	}
	if req.EventCategory != "" {
		query.EventCategory = &req.EventCategory
	}
	if req.EventType != "" {
		query.EventType = &req.EventType
	}
	if req.Severity != "" {
		query.Severity = &req.Severity
	}
	if req.Status != "" {
		query.Status = &req.Status
	}
	if req.StartDate != nil {
		startDate := req.StartDate.AsTime()
		query.StartDate = &startDate
	}
	if req.EndDate != nil {
		endDate := req.EndDate.AsTime()
		query.EndDate = &endDate
	}

	logs, err := s.auditService.QueryAuditLogs(ctx, query)
	if err != nil {
		return nil, err
	}

	protoLogs := make([]*auditpb.SecurityAuditLog, 0, len(logs))
	for _, log := range logs {
		protoLogs = append(protoLogs, toProtoSecurityAuditLog(log))
	}

	return &auditpb.QueryAuditLogsResponse{
		Logs:       protoLogs,
		TotalCount: int32(len(protoLogs)),
	}, nil
}

func (s *SecurityAuditGRPCServer) GetCriticalEvents(ctx context.Context, req *auditpb.GetCriticalEventsRequest) (*auditpb.GetCriticalEventsResponse, error) {
	hours := int(req.Hours)
	if hours <= 0 {
		hours = 24
	}

	limit := int(req.Limit)
	if limit <= 0 {
		limit = 50
	}

	events, err := s.auditService.GetCriticalEvents(ctx, hours, limit)
	if err != nil {
		return nil, err
	}

	protoEvents := make([]*auditpb.SecurityAuditLog, 0, len(events))
	for _, event := range events {
		protoEvents = append(protoEvents, toProtoSecurityAuditLog(event))
	}

	return &auditpb.GetCriticalEventsResponse{
		Events: protoEvents,
	}, nil
}

func (s *SecurityAuditGRPCServer) GetSecuritySummary(ctx context.Context, req *auditpb.GetSecuritySummaryRequest) (*auditpb.GetSecuritySummaryResponse, error) {
	startDate := req.StartDate.AsTime()
	endDate := req.EndDate.AsTime()

	summary, err := s.auditService.GetSecuritySummary(ctx, startDate, endDate)
	if err != nil {
		return nil, err
	}

	protoEvents := make([]*auditpb.SecurityEventsSummary, 0, len(summary.Events))
	for _, event := range summary.Events {
		protoEvents = append(protoEvents, &auditpb.SecurityEventsSummary{
			EventDate:     timestamppb.New(event.EventDate),
			EventCategory: event.EventCategory,
			EventType:     event.EventType,
			Status:        event.Status,
			Severity:      event.Severity,
			EventCount:    event.EventCount,
			UniqueUsers:   event.UniqueUsers,
			UniqueIps:     event.UniqueIPs,
		})
	}

	protoHighRisk := make([]*auditpb.SuspiciousActivity, 0, len(summary.HighRiskUsers))
	for _, activity := range summary.HighRiskUsers {
		protoHighRisk = append(protoHighRisk, toProtoSuspiciousActivity(activity))
	}

	return &auditpb.GetSecuritySummaryResponse{
		StartDate:      timestamppb.New(summary.StartDate),
		EndDate:        timestamppb.New(summary.EndDate),
		TotalEvents:    summary.TotalEvents,
		TotalFailures:  summary.TotalFailures,
		CriticalEvents: summary.CriticalEvents,
		Events:         protoEvents,
		HighRiskUsers:  protoHighRisk,
	}, nil
}

func (s *SecurityAuditGRPCServer) GetUserRiskScore(ctx context.Context, req *auditpb.GetUserRiskScoreRequest) (*auditpb.GetUserRiskScoreResponse, error) {
	riskScore, err := s.auditService.GetUserRiskScore(ctx, req.UserId)
	if err != nil {
		return nil, err
	}

	return &auditpb.GetUserRiskScoreResponse{
		UserId:    req.UserId,
		RiskScore: int32(riskScore),
	}, nil
}
func (s *SecurityAuditGRPCServer) GetUsersRiskScores(ctx context.Context, req *auditpb.GetUsersRiskScoresRequest) (*auditpb.GetUsersRiskScoresResponse, error) {
	scores := make([]*auditpb.UserRiskScore, 0, len(req.UserIds))

	for _, userID := range req.UserIds {
		riskScore, err := s.auditService.GetUserRiskScore(ctx, userID)
		if err != nil {
			// Log error but continue
			scores = append(scores, &auditpb.UserRiskScore{
				UserId:    userID,
				RiskScore: 0,
			})
			continue
		}

		scores = append(scores, &auditpb.UserRiskScore{
			UserId:    userID,
			RiskScore: int32(riskScore),
		})
	}

	return &auditpb.GetUsersRiskScoresResponse{
		Scores: scores,
	}, nil
}

// ================================
// RATE LIMITING
// ================================

func (s *SecurityAuditGRPCServer) CheckRateLimit(ctx context.Context, req *auditpb.CheckRateLimitRequest) (*auditpb.CheckRateLimitResponse, error) {
	isLimited, err := s.auditService.CheckRateLimit(ctx, req.Identifier, req.IpAddress)
	if err != nil {
		return nil, err
	}

	return &auditpb.CheckRateLimitResponse{
		IsLimited: isLimited,
		Message:   determineRateLimitMessage(isLimited),
	}, nil
}

func (s *SecurityAuditGRPCServer) CheckRateLimits(ctx context.Context, req *auditpb.CheckRateLimitsRequest) (*auditpb.CheckRateLimitsResponse, error) {
	results := make([]*auditpb.RateLimitResult, 0, len(req.Checks))

	for _, check := range req.Checks {
		isLimited, err := s.auditService.CheckRateLimit(ctx, check.Identifier, check.IpAddress)
		
		result := &auditpb.RateLimitResult{
			Identifier: check.Identifier,
			IpAddress:  check.IpAddress,
			IsLimited:  false,
		}

		if err != nil {
			result.Error = err.Error()
		} else {
			result.IsLimited = isLimited
		}

		results = append(results, result)
	}

	return &auditpb.CheckRateLimitsResponse{
		Results: results,
	}, nil
}

// ================================
// HELPER CONVERTERS
// ================================

func toProtoAccountLockout(lockout *domain.AccountLockout) *auditpb.AccountLockout {
	proto := &auditpb.AccountLockout{
		Id:       lockout.ID,
		UserId:   lockout.UserID,
		Reason:   lockout.Reason,
		LockedAt: timestamppb.New(lockout.LockedAt),
		IsActive: lockout.IsActive,
	}

	if lockout.LockedBy != nil {
		proto.LockedBy = *lockout.LockedBy
	}
	if lockout.UnlockAt != nil {
		proto.UnlockAt = timestamppb.New(*lockout.UnlockAt)
	}
	if lockout.UnlockedAt != nil {
		proto.UnlockedAt = timestamppb.New(*lockout.UnlockedAt)
	}
	if lockout.UnlockedBy != nil {
		proto.UnlockedBy = *lockout.UnlockedBy
	}
	if lockout.Metadata != nil {
		proto.Metadata, _ = structpb.NewStruct(lockout.Metadata)
	}

	return proto
}

func toProtoSuspiciousActivity(activity *domain.SuspiciousActivity) *auditpb.SuspiciousActivity {
	proto := &auditpb.SuspiciousActivity{
		Id:           activity.ID,
		UserId:       activity.UserID,
		ActivityType: activity.ActivityType,
		RiskScore:    int32(activity.RiskScore),
		Status:       activity.Status,
		CreatedAt:    timestamppb.New(activity.CreatedAt),
		UpdatedAt:    timestamppb.New(activity.UpdatedAt),
	}

	if activity.IPAddress != nil {
		proto.IpAddress = *activity.IPAddress
	}
	if activity.Details != nil {
		proto.Details, _ = structpb.NewStruct(activity.Details)
	}
	if activity.ResolvedBy != nil {
		proto.ResolvedBy = *activity.ResolvedBy
	}
	if activity.ResolvedAt != nil {
		proto.ResolvedAt = timestamppb.New(*activity.ResolvedAt)
	}

	return proto
}

func toProtoSecurityAuditLog(log *domain.SecurityAuditLog) *auditpb.SecurityAuditLog {
	proto := &auditpb.SecurityAuditLog{
		Id:            log.ID,
		EventType:     log.EventType,
		EventCategory: log.EventCategory,
		Severity:      log.Severity,
		Status:        log.Status,
		CreatedAt:     timestamppb.New(log.CreatedAt),
	}

	if log.UserID != nil {
		proto.UserId = *log.UserID
	}
	if log.TargetUserID != nil {
		proto.TargetUserId = *log.TargetUserID
	}
	if log.SessionID != nil {
		proto.SessionId = *log.SessionID
	}
	if log.IPAddress != nil {
		proto.IpAddress = *log.IPAddress
	}
	if log.UserAgent != nil {
		proto.UserAgent = *log.UserAgent
	}
	if log.RequestID != nil {
		proto.RequestId = *log.RequestID
	}
	if log.ResourceType != nil {
		proto.ResourceType = *log.ResourceType
	}
	if log.ResourceID != nil {
		proto.ResourceId = *log.ResourceID
	}
	if log.Action != nil {
		proto.Action = *log.Action
	}
	if log.Description != nil {
		proto.Description = *log.Description
	}
	if log.Metadata != nil {
		proto.Metadata, _ = structpb.NewStruct(log.Metadata)
	}
	if log.PreviousValue != nil {
		proto.PreviousValue, _ = structpb.NewStruct(log.PreviousValue)
	}
	if log.NewValue != nil {
		proto.NewValue, _ = structpb.NewStruct(log.NewValue)
	}
	if log.ErrorCode != nil {
		proto.ErrorCode = *log.ErrorCode
	}
	if log.ErrorMessage != nil {
		proto.ErrorMessage = *log.ErrorMessage
	}
	if log.Country != nil {
		proto.Country = *log.Country
	}
	if log.City != nil {
		proto.City = *log.City
	}

	return proto
}

func determineRateLimitMessage(isLimited bool) string {
	if isLimited {
		return "Rate limit exceeded. Please try again later."
	}
	return "Within rate limit"
}