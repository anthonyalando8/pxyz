// internal/risk/risk_assessor.go
package risk

import (
    "context"
    "crypto-service/internal/repository"
    "crypto-service/internal/domain"
    "math/big"
    "time"
    
    "go.uber.org/zap"
)

type RiskAssessor struct {
    transactionRepo *repository.CryptoTransactionRepository
    walletRepo      *repository.CryptoWalletRepository
    logger          *zap.Logger
}

func NewRiskAssessor(
    transactionRepo *repository.CryptoTransactionRepository,
    walletRepo      *repository.CryptoWalletRepository,
    logger *zap.Logger,
) *RiskAssessor {
    return &RiskAssessor{
        transactionRepo: transactionRepo,
        walletRepo:      walletRepo,
        logger:          logger,
    }
}

type RiskAssessment struct {
    RiskScore      int
    RiskFactors    []domain.RiskFactor
    RequiresApproval bool
    Explanation    string
}



// AssessWithdrawal evaluates withdrawal risk
func (r *RiskAssessor) AssessWithdrawal(
    ctx context.Context,
    userID, chain, asset string,
    amount *big.Int,
    toAddress string,
) (*RiskAssessment, error) {
    
    assessment := &RiskAssessment{
        RiskScore:   0,
        RiskFactors: []domain.RiskFactor{},
    }
    
    //  1. Check withdrawal amount (high amount = higher risk)
    if riskFactor := r.assessAmountRisk(chain, asset, amount); riskFactor != nil {
        assessment.RiskFactors = append(assessment.RiskFactors, *riskFactor)
        assessment.RiskScore += riskFactor.Score
    }
    
    //  2. Check destination address (new address = higher risk)
    if riskFactor := r.assessAddressRisk(ctx, userID, chain, toAddress); riskFactor != nil {
        assessment.RiskFactors = append(assessment.RiskFactors, *riskFactor)
        assessment.RiskScore += riskFactor.Score
    }
    
    //  3. Check user transaction history (new user = higher risk)
    if riskFactor := r.assessUserHistoryRisk(ctx, userID, chain, asset); riskFactor != nil {
        assessment.RiskFactors = append(assessment.RiskFactors, *riskFactor)
        assessment.RiskScore += riskFactor.Score
    }
    
    //  4. Check withdrawal frequency (too many = higher risk)
    if riskFactor := r.assessFrequencyRisk(ctx, userID, chain, asset); riskFactor != nil {
        assessment.RiskFactors = append(assessment.RiskFactors, *riskFactor)
        assessment.RiskScore += riskFactor.Score
    }
    
    //  5. Check if address is on blacklist (instant high risk)
    if riskFactor := r.assessBlacklistRisk(ctx, toAddress); riskFactor != nil {
        assessment.RiskFactors = append(assessment.RiskFactors, *riskFactor)
        assessment.RiskScore += riskFactor.Score
    }
    
    //  Determine if approval is required
    // Risk score 0-30: Auto-approve
    // Risk score 31-60: Review recommended
    // Risk score 61+: Approval required
    
    if assessment.RiskScore >= 61 {
        assessment.RequiresApproval = true
        assessment.Explanation = "High risk - manual approval required"
    } else if assessment.RiskScore >= 31 {
        assessment.RequiresApproval = true
        assessment.Explanation = "Medium risk - review recommended"
    } else {
        assessment.RequiresApproval = false
        assessment.Explanation = "Low risk - auto-approved"
    }
    
    r.logger.Info("Withdrawal risk assessment completed",
        zap.String("user_id", userID),
        zap.Int("risk_score", assessment.RiskScore),
        zap.Bool("requires_approval", assessment.RequiresApproval),
        zap.Int("factors_count", len(assessment.RiskFactors)))
    
    return assessment, nil
}

// assessAmountRisk checks if withdrawal amount is unusually high
func (r *RiskAssessor) assessAmountRisk(chain, asset string, amount *big.Int) *domain.RiskFactor {
    // Define thresholds (in USD equivalent)
    thresholds := map[string]map[string]int64{
        "TRON": {
            "TRX":  100000000000, // 100,000 TRX (~$10,000)
            "USDT": 10000000000,  // 10,000 USDT
        },
        "BITCOIN": {
            "BTC": 50000000, // 0.5 BTC (~$20,000)
        },
        "ETHEREUM": {
            "ETH":  5000000000000000000, // 5 ETH (~$10,000)
            "USDC": 10000000000,         // 10,000 USDC
        },
    }
    
    var threshold int64 = 10000000000 // Default threshold
    if chainThresholds, ok := thresholds[chain]; ok {
        if t, ok := chainThresholds[asset]; ok {
            threshold = t
        }
    }
    
    thresholdBig := big.NewInt(threshold)
    
    // High amount (>2x threshold)
    doubleThreshold := new(big.Int).Mul(thresholdBig, big.NewInt(2))
    if amount.Cmp(doubleThreshold) > 0 {
        return &domain.RiskFactor{
            Factor:      "large_amount",
            Description: "Withdrawal amount is very high",
            Score:       40,
        }
    }
    
    // Medium amount (>threshold)
    if amount.Cmp(thresholdBig) > 0 {
        return &domain.RiskFactor{
            Factor:      "medium_amount",
            Description: "Withdrawal amount is above threshold",
            Score:       20,
        }
    }
    
    return nil
}

// assessAddressRisk checks if destination address is known/trusted
func (r *RiskAssessor) assessAddressRisk(ctx context.Context, userID, chain, toAddress string) *domain.RiskFactor {
    // Check if address is in user's address book
    // (You'll need to implement this query in repository)
    // For now, simplified check:
    
    // Check if user has sent to this address before
    previousTx, err := r.transactionRepo.GetUserTransactionToAddress(ctx, userID, chain, toAddress)
    if err == nil && previousTx != nil {
        // Known address - low risk
        return nil
    }
    
    // New address - medium risk
    return &domain.RiskFactor{
        Factor:      "new_address",
        Description: "First time sending to this address",
        Score:       25,
    }
}

// assessUserHistoryRisk checks user's transaction history
func (r *RiskAssessor) assessUserHistoryRisk(ctx context.Context, userID, chain, asset string) *domain.RiskFactor {
    // Get user's successful withdrawals count
    count, err := r.transactionRepo.GetUserWithdrawalCount(ctx, userID, chain, asset)
    if err != nil {
        return nil
    }
    
    // New user (0-2 withdrawals) = higher risk
    if count <= 2 {
        return &domain.RiskFactor{
            Factor:      "new_user",
            Description: "Limited withdrawal history",
            Score:       30,
        }
    }
    
    // Moderate history (3-10 withdrawals) = medium risk
    if count <= 10 {
        return &domain.RiskFactor{
            Factor:      "limited_history",
            Description: "Limited transaction history",
            Score:       15,
        }
    }
    
    // Established user (10+ withdrawals) = low risk
    return nil
}

// assessFrequencyRisk checks withdrawal frequency
func (r *RiskAssessor) assessFrequencyRisk(ctx context.Context, userID, chain, asset string) *domain.RiskFactor {
    // Get withdrawals in last 24 hours
    since := time.Now().Add(-24 * time.Hour)
    recentCount, err := r.transactionRepo.GetUserWithdrawalsSince(ctx, userID, chain, asset, since)
    if err != nil {
        return nil
    }
    
    // More than 5 withdrawals in 24h = suspicious
    if recentCount > 5 {
        return &domain.RiskFactor{
            Factor:      "high_frequency",
            Description: "Multiple withdrawals in short time",
            Score:       35,
        }
    }
    
    // 3-5 withdrawals = moderate
    if recentCount >= 3 {
        return &domain.RiskFactor{
            Factor:      "moderate_frequency",
            Description: "Several recent withdrawals",
            Score:       15,
        }
    }
    
    return nil
}

// assessBlacklistRisk checks if address is blacklisted
func (r *RiskAssessor) assessBlacklistRisk(ctx context.Context, address string) *domain.RiskFactor {
    // TODO: Check against blacklist database
    // For now, simplified check
    
    // You can integrate with services like:
    // - Chainalysis
    // - Elliptic
    // - Internal blacklist
    
    return nil // No blacklist check for now
}