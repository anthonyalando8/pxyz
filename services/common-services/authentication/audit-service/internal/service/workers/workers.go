// security_audit_workers.go
package workers

import (
	"audit-service/internal/domain"
	"audit-service/internal/service/audit"
	"audit-service/internal/service/ws"
	"context"
	"log"
	"time"
)

// ================================
// WORKER COORDINATOR
// ================================

type SecurityAuditWorkers struct {
	auditService *service.SecurityAuditService
	notifier     *websocket.SecurityAuditNotifier
	ctx          context.Context
	cancel       context.CancelFunc
}

func NewSecurityAuditWorkers(
	auditService *service.SecurityAuditService,
	notifier *websocket.SecurityAuditNotifier,
) *SecurityAuditWorkers {
	ctx, cancel := context.WithCancel(context.Background())
	return &SecurityAuditWorkers{
		auditService: auditService,
		notifier:     notifier,
		ctx:          ctx,
		cancel:       cancel,
	}
}

// Start starts all background workers
func (w *SecurityAuditWorkers) Start() {
	log.Println("Starting security audit workers...")

	// Start maintenance worker (runs every hour)
	go w.runMaintenanceWorker(1 * time.Hour)

	// Start suspicious activity detector (runs every 5 minutes)
	go w.runSuspiciousActivityDetector(5 * time.Minute)

	// Start auto-resolve worker (runs every 15 minutes)
	go w.runAutoResolveWorker(15 * time.Minute)

	// Start critical event monitor (runs every 1 minute)
	go w.runCriticalEventMonitor(1 * time.Minute)

	// Start high-risk user notifier (runs every 5 minutes)
	go w.runHighRiskUserNotifier(5 * time.Minute)

	log.Println("All security audit workers started successfully")
}

// Stop stops all background workers
func (w *SecurityAuditWorkers) Stop() {
	log.Println("Stopping security audit workers...")
	w.cancel()
}

// ================================
// MAINTENANCE WORKER
// ================================

func (w *SecurityAuditWorkers) runMaintenanceWorker(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	log.Printf("Maintenance worker started (interval: %v)", interval)

	// Run immediately on startup
	w.runMaintenance()

	for {
		select {
		case <-ticker.C:
			w.runMaintenance()

		case <-w.ctx.Done():
			log.Println("Maintenance worker stopped")
			return
		}
	}
}

func (w *SecurityAuditWorkers) runMaintenance() {
	log.Println("Running maintenance tasks...")

	report, err := w.auditService.RunMaintenance(w.ctx)
	if err != nil {
		log.Printf("Maintenance failed: %v", err)
		return
	}

	log.Printf("Maintenance completed: unlocked=%d, cleaned_logins=%d, cleaned_audits=%d, duration=%v",
		report.UnlockedAccounts,
		report.CleanedFailedLogins,
		report.CleanedAuditLogs,
		report.Duration,
	)

	if len(report.Errors) > 0 {
		log.Printf("Maintenance errors: %v", report.Errors)
	}
}

// ================================
// SUSPICIOUS ACTIVITY DETECTOR
// ================================

func (w *SecurityAuditWorkers) runSuspiciousActivityDetector(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	log.Printf("Suspicious activity detector started (interval: %v)", interval)

	for {
		select {
		case <-ticker.C:
			w.detectSuspiciousActivities()

		case <-w.ctx.Done():
			log.Println("Suspicious activity detector stopped")
			return
		}
	}
}

func (w *SecurityAuditWorkers) detectSuspiciousActivities() {
	log.Println("Detecting suspicious activities...")

	// Get recent failed logins grouped by user
	// This is a simplified example - you'd implement more sophisticated detection

	// Query for users with multiple failed logins in the last hour
	query := &domain.AuditLogQuery{
		EventType: stringPtr(domain.EventLoginFailed),
		Status:    stringPtr(domain.StatusFailure),
		StartDate: timePtr(time.Now().Add(-1 * time.Hour)),
		Limit:     1000,
	}

	logs, err := w.auditService.QueryAuditLogs(w.ctx, query)
	if err != nil {
		log.Printf("Failed to query audit logs: %v", err)
		return
	}

	// Group by user and analyze
	userFailures := make(map[string]int)
	for _, logEntry := range logs {
		if logEntry.UserID != nil {
			userFailures[*logEntry.UserID]++
		}
	}

	// Report suspicious activity for users with many failed logins
	for userID, count := range userFailures {
		if count >= 5 {
			req := &domain.ReportSuspiciousActivityRequest{
				UserID:       userID,
				ActivityType: "multiple_failed_logins",
				RiskScore:    min(count*10, 100),
				Details: map[string]interface{}{
					"failed_login_count": count,
					"time_window":        "1 hour",
					"detected_by":        "automated_worker",
				},
			}

			if err := w.auditService.ReportSuspiciousActivity(w.ctx, req, "system"); err != nil {
				log.Printf("Failed to report suspicious activity for user %s: %v", userID, err)
			} else {
				log.Printf("Reported suspicious activity for user %s (failed logins: %d)", userID, count)
			}
		}
	}
}

// ================================
// AUTO-RESOLVE WORKER
// ================================

func (w *SecurityAuditWorkers) runAutoResolveWorker(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	log.Printf("Auto-resolve worker started (interval: %v)", interval)

	for {
		select {
		case <-ticker.C:
			w.autoResolveSuspiciousActivities()

		case <-w.ctx.Done():
			log.Println("Auto-resolve worker stopped")
			return
		}
	}
}

func (w *SecurityAuditWorkers) autoResolveSuspiciousActivities() {
	log.Println("Auto-resolving old suspicious activities...")

	// This is a placeholder - implement your auto-resolve logic
	// For example: resolve activities older than 30 days with no further incidents

	// You might want to:
	// 1. Get activities older than 30 days that are still active
	// 2. Check if the user has had any incidents in the last 7 days
	// 3. If clean, auto-resolve as false_positive or resolved

	log.Println("Auto-resolve completed")
}

// ================================
// CRITICAL EVENT MONITOR
// ================================

func (w *SecurityAuditWorkers) runCriticalEventMonitor(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	log.Printf("Critical event monitor started (interval: %v)", interval)

	for {
		select {
		case <-ticker.C:
			w.monitorCriticalEvents()

		case <-w.ctx.Done():
			log.Println("Critical event monitor stopped")
			return
		}
	}
}

func (w *SecurityAuditWorkers) monitorCriticalEvents() {
	// Get critical events from the last 5 minutes
	events, err := w.auditService.GetCriticalEvents(w.ctx, 1, 50) // Last 1 hour, limit 50
	if err != nil {
		log.Printf("Failed to get critical events: %v", err)
		return
	}

	// Filter to only very recent events (last 2 minutes to avoid duplicates)
	cutoff := time.Now().Add(-2 * time.Minute)
	for _, event := range events {
		if event.CreatedAt.After(cutoff) {
			// Send WebSocket notification
			if w.notifier != nil {
				if err := w.notifier.NotifyCriticalEvent(w.ctx, event); err != nil {
					log.Printf("Failed to notify critical event: %v", err)
				}
			}
		}
	}
}

// ================================
// HIGH-RISK USER NOTIFIER
// ================================

func (w *SecurityAuditWorkers) runHighRiskUserNotifier(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	log.Printf("High-risk user notifier started (interval: %v)", interval)

	for {
		select {
		case <-ticker.C:
			w.notifyHighRiskUsers()

		case <-w.ctx.Done():
			log.Println("High-risk user notifier stopped")
			return
		}
	}
}

func (w *SecurityAuditWorkers) notifyHighRiskUsers() {
	if w.notifier == nil {
		return
	}

	if err := w.notifier.NotifyHighRiskUsers(w.ctx); err != nil {
		log.Printf("Failed to notify high-risk users: %v", err)
	}
}

// ================================
// SCHEDULED TASKS
// ================================

// RunDailyTasks runs daily maintenance tasks (call this via cron or scheduler)
func (w *SecurityAuditWorkers) RunDailyTasks() {
	log.Println("Running daily tasks...")

	ctx := context.Background()

	// 1. Refresh materialized views
	if err := w.auditService.RefreshSecurityEventsSummary(ctx); err != nil {
		log.Printf("Failed to refresh security events summary: %v", err)
	}

	// 2. Generate and log daily security summary
	startDate := time.Now().Add(-24 * time.Hour)
	endDate := time.Now()

	summary, err := w.auditService.GetSecuritySummary(ctx, startDate, endDate)
	if err != nil {
		log.Printf("Failed to get security summary: %v", err)
	} else {
		log.Printf("Daily Summary: total=%d, failures=%d, critical=%d, high_risk_users=%d",
			summary.TotalEvents,
			summary.TotalFailures,
			summary.CriticalEvents,
			len(summary.HighRiskUsers),
		)
	}

	// 3. Check for accounts that should be permanently deleted
	// (soft-deleted > 30 days ago per GDPR requirements)
	// deletedCount, err := w.auditService.PermanentlyDeleteUsers(ctx, 30)
	// if err != nil {
	// 	log.Printf("Failed to permanently delete users: %v", err)
	// } else {
	// 	log.Printf("Permanently deleted %d users", deletedCount)
	// }

	log.Println("Daily tasks completed")
}

// RunWeeklyTasks runs weekly maintenance tasks
func (w *SecurityAuditWorkers) RunWeeklyTasks() {
	log.Println("Running weekly tasks...")

	ctx := context.Background()

	// Generate weekly security report
	startDate := time.Now().Add(-7 * 24 * time.Hour)
	endDate := time.Now()

	summary, err := w.auditService.GetSecuritySummary(ctx, startDate, endDate)
	if err != nil {
		log.Printf("Failed to get weekly security summary: %v", err)
	} else {
		log.Printf("Weekly Summary: total=%d, failures=%d, critical=%d",
			summary.TotalEvents,
			summary.TotalFailures,
			summary.CriticalEvents,
		)

		// You could send this as an email report to admins
		// or store it in a weekly_reports table
	}

	log.Println("Weekly tasks completed")
}

// ================================
// HELPER FUNCTIONS
// ================================

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func stringPtr(s string) *string {
	return &s
}

func timePtr(t time.Time) *time.Time {
	return &t
}

// ================================
// SCHEDULER INTEGRATION EXAMPLE
// ================================

/*
Example using robfig/cron for scheduling:

import "github.com/robfig/cron/v3"

func SetupScheduledTasks(workers *SecurityAuditWorkers) {
	c := cron.New()

	// Daily tasks at 2 AM
	c.AddFunc("0 2 * * *", func() {
		workers.RunDailyTasks()
	})

	// Weekly tasks on Monday at 3 AM
	c.AddFunc("0 3 * * MON", func() {
		workers.RunWeeklyTasks()
	})

	c.Start()
}
*/
