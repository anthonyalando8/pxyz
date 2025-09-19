package server

import (
	"account-service/internal/config"
	"account-service/internal/handler/2fa"
	acchandler "account-service/internal/handler/acc"
	prefhandler "account-service/internal/handler/prefs"
	"account-service/internal/repository"
	"account-service/internal/service/2fa"
	acservice "account-service/internal/service/acc"
	prefservice "account-service/internal/service/prefs"
	notificationclient "x/shared/notification" // ✅ added
	"log"
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	emailclient "x/shared/email"
	pb "x/shared/genproto/accountpb"
	"x/shared/utils/id"
)

type Server struct {
	pb.UnimplementedAccountServiceServer
	Cfg  config.Config
	DB   *pgxpool.Pool
	Rdb  *redis.Client

	twofaHandler *_2fahandler.TwoFAHandler
	accHandler   *acchandler.AccountHandler
	prefHandler  *prefhandler.PreferencesHandler
}

func NewServer() *Server {
	cfg := config.Load()

	// DB
	dbpool, err := pgxpool.New(context.Background(), cfg.DBConnString)
	if err != nil {
		log.Fatalf("[FATAL] failed to connect to DB: %v", err)
	}

	// Redis
	rdb := redis.NewClient(&redis.Options{
		Addr: cfg.RedisAddr, Password: cfg.RedisPass,
	})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		log.Printf("[WARN] failed to connect to Redis: %v", err)
	}

	// Snowflake
	sf, err := id.NewSnowflake(7)
	if err != nil {
		log.Fatalf("[FATAL] snowflake init failed: %v", err)
	}

	emailCli := emailclient.NewEmailClient()
	notificationCli := notificationclient.NewNotificationService() // ✅ create notification client


	// 2FA wiring
	_2faRepo := repository.NewTwoFARepository(dbpool)
	_2faUc := _2faservice.NewTwoFAService(_2faRepo, sf)
	twofaHandler := _2fahandler.NewTwoFAHandler(_2faUc, emailCli, notificationCli)

	// Account wiring
	accRepo := repository.NewUserProfileRepository(dbpool)
	accUc := acservice.NewAccountService(accRepo, sf)
	accHandler := acchandler.NewAccountHandler(accUc, emailCli)

	// Preferences wiring
	prefRepo := repository.NewPreferencesRepository(dbpool)
	prefSvc := prefservice.NewPreferencesService(prefRepo, sf)
	prefHandler := prefhandler.NewPreferencesHandler(prefSvc)

	return &Server{
		Cfg:          cfg,
		DB:           dbpool,
		Rdb:          rdb,
		twofaHandler: twofaHandler,
		accHandler:   accHandler,
		prefHandler:  prefHandler,
	}
}



func (s *Server) InitiateTOTPSetup(ctx context.Context, req *pb.InitiateTOTPSetupRequest) (*pb.InitiateTOTPSetupResponse, error) {
    return s.twofaHandler.InitiateTOTPSetup(ctx, req)
}

func (s *Server) EnableTwoFA(ctx context.Context, req *pb.EnableTwoFARequest) (*pb.EnableTwoFAResponse, error) {
	return s.twofaHandler.EnableTwoFA(ctx, req)
}

// ---------- Get 2FA Status ----------
func (s *Server) GetTwoFAStatus(ctx context.Context, req *pb.GetTwoFAStatusRequest) (*pb.GetTwoFAStatusResponse, error) {
	return s.twofaHandler.GetTwoFAStatus(ctx, req)
}

// ---------- Verify 2FA ----------
func (s *Server) VerifyTwoFA(ctx context.Context, req *pb.VerifyTwoFARequest) (*pb.VerifyTwoFAResponse, error) {
	return s.twofaHandler.VerifyTwoFA(ctx, req)
}

// ---------- Disable 2FA ----------
func (s *Server) DisableTwoFA(ctx context.Context, req *pb.DisableTwoFARequest) (*pb.DisableTwoFAResponse, error) {
	return s.twofaHandler.DisableTwoFA(ctx, req)
}

// ---------- Regenerate Backup Codes ----------
func (s *Server) RegenerateBackupCodes(ctx context.Context, req *pb.RegenerateBackupCodesRequest) (*pb.RegenerateBackupCodesResponse, error) {
	return s.twofaHandler.RegenerateBackupCodes(ctx, req)
}

// ---------- Get Backup Codes ----------
func (s *Server) GetBackupCodes(ctx context.Context, req *pb.GetBackupCodesRequest) (*pb.GetBackupCodesResponse, error) {
	return s.twofaHandler.GetBackupCodes(ctx, req)
}

func (s *Server) GetUserProfile(ctx context.Context, req *pb.GetUserProfileRequest) (*pb.GetUserProfileResponse, error) {
    return s.accHandler.GetUserProfile(ctx, req)
}

func (s *Server) UpdateProfile(ctx context.Context, req *pb.UpdateProfileRequest) (*pb.UpdateProfileResponse, error) {
    return s.accHandler.UpdateAccountHandler(ctx, req)
}

func (s *Server) UpdateProfilePicture(ctx context.Context, req *pb.UpdateProfilePictureRequest) (*pb.UpdateProfilePictureResponse,error) {
    return s.accHandler.UpdateProfilePicture(ctx, req)
}

func (s *Server) UpdateUserNationality(ctx context.Context, req *pb.UpdateUserNationalityRequest) (*pb.UpdateUserNationalityResponse, error) {
    return s.accHandler.UpdateUserNationality(ctx, req)
}

func (s *Server) GetUserNationality(ctx context.Context, req *pb.GetUserNationalityRequest) (*pb.GetUserNationalityResponse, error) {
    return s.accHandler.GetUserNationalityStatus(ctx, req)
}

// Preferences RPCs

func (s *Server) GetPreferences(ctx context.Context, req *pb.GetPreferencesRequest) (*pb.GetPreferencesResponse, error) {
	return s.prefHandler.GetPreferences(ctx, req)
}

func (s *Server) UpdatePreferences(ctx context.Context, req *pb.UpdatePreferencesRequest) (*pb.UpdatePreferencesResponse, error) {
	return s.prefHandler.UpdatePreferences(ctx, req)
}

