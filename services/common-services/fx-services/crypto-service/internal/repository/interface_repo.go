// internal/repository/crypto_wallet_repository.go
package repository

// import (
// 	"context"
// 	"crypto-service/internal/domain"
// 	"math/big"
// 	"time"
// )

// // type CryptoWalletRepository interface {
// // 	// Core CRUD
// // 	Create(ctx context.Context, wallet *domain.CryptoWallet) error
// // 	GetByID(ctx context.Context, id int64) (*domain.CryptoWallet, error)
// // 	GetByAddress(ctx context.Context, address string) (*domain.CryptoWallet, error)
// // 	Update(ctx context.Context, wallet *domain.CryptoWallet) error
// // 	Delete(ctx context. Context, id int64) error
	
// // 	// User wallets
// // 	GetUserWallets(ctx context.Context, userID string) ([]*domain.CryptoWallet, error)
// // 	GetUserWalletByChainAsset(ctx context.Context, userID, chain, asset string) (*domain.CryptoWallet, error)
// // 	GetUserPrimaryWallet(ctx context.Context, userID, chain, asset string) (*domain.CryptoWallet, error)
	
// // 	// Balance operations
// // 	UpdateBalance(ctx context.Context, walletID int64, balance *big.Int) error
// // 	GetWalletBalance(ctx context.Context, walletID int64) (*domain.WalletBalance, error)
	
// // 	// Monitoring
// // 	GetWalletsForDepositCheck(ctx context.Context, limit int) ([]*domain.CryptoWallet, error)
// // 	UpdateLastDepositCheck(ctx context. Context, walletID int64) error
// // 	UpdateLastTransactionBlock(ctx context. Context, walletID int64, blockNumber int64) error
	
// // 	// Batch operations
// // 	GetWalletsByChain(ctx context.Context, chain string) ([]*domain.CryptoWallet, error)
// // 	GetActiveWallets(ctx context.Context) ([]*domain.CryptoWallet, error)
// // }



// // type CryptoTransactionRepository interface {
// // 	// Core CRUD
// // 	Create(ctx context.Context, tx *domain.CryptoTransaction) error
// // 	GetByID(ctx context.Context, id int64) (*domain.CryptoTransaction, error)
// // 	GetByTransactionID(ctx context.Context, txID string) (*domain.CryptoTransaction, error)
// // 	GetByTxHash(ctx context.Context, txHash string) (*domain.CryptoTransaction, error)
// // 	Update(ctx context.Context, tx *domain.CryptoTransaction) error
	
// // 	// Status updates
// // 	UpdateStatus(ctx context. Context, id int64, status domain.TransactionStatus, message *string) error
// // 	UpdateConfirmations(ctx context.Context, id int64, confirmations int) error
// // 	MarkAsBroadcasted(ctx context.Context, id int64, txHash string) error
// // 	MarkAsConfirmed(ctx context.Context, id int64, blockNumber int64, blockTime time.Time) error
// // 	MarkAsFailed(ctx context. Context, id int64, reason string) error
	
// // 	// Queries
// // 	GetUserTransactions(ctx context.Context, userID string, limit, offset int) ([]*domain.CryptoTransaction, error)
// // 	GetPendingTransactions(ctx context.Context) ([]*domain.CryptoTransaction, error)
// // 	GetTransactionsByStatus(ctx context.Context, status domain.TransactionStatus) ([]*domain.CryptoTransaction, error)
// // 	GetRecentTransactions(ctx context.Context, limit int) ([]*domain.TransactionSummary, error)
	
// // 	// Internal transfers
// // 	GetInternalTransfers(ctx context.Context, userID string) ([]*domain.CryptoTransaction, error)
	
// // 	// Statistics
// // 	GetTransactionCount(ctx context.Context, userID string) (int64, error)
// // 	GetTransactionVolume(ctx context.Context, userID, chain, asset string, from, to time.Time) (*big.Int, error)
// // }

// // internal/repository/crypto_deposit_repository.go


// type CryptoDepositRepository interface {
// 	// Core CRUD
// 	Create(ctx context.Context, deposit *domain.CryptoDeposit) error
// 	GetByID(ctx context.Context, id int64) (*domain.CryptoDeposit, error)
// 	GetByDepositID(ctx context.Context, depositID string) (*domain.CryptoDeposit, error)
// 	GetByTxHash(ctx context.Context, txHash, toAddress string) (*domain.CryptoDeposit, error)
// 	Update(ctx context.Context, deposit *domain.CryptoDeposit) error
	
// 	// Status updates
// 	UpdateStatus(ctx context.Context, id int64, status domain.DepositStatus) error
// 	UpdateConfirmations(ctx context.Context, id int64, confirmations int) error
// 	MarkAsConfirmed(ctx context.Context, id int64) error
// 	MarkAsCredited(ctx context.Context, id int64, transactionID int64) error
// 	MarkAsNotified(ctx context.Context, id int64) error
	
// 	// Queries
// 	GetPendingDeposits(ctx context.Context) ([]*domain.CryptoDeposit, error)
// 	GetUserDeposits(ctx context.Context, userID string, limit, offset int) ([]*domain.CryptoDeposit, error)
// 	GetWalletDeposits(ctx context.Context, walletID int64) ([]*domain.CryptoDeposit, error)
// 	GetUnnotifiedDeposits(ctx context. Context) ([]*domain.CryptoDeposit, error)
	
// 	// Duplicate check
// 	DepositExists(ctx context.Context, txHash, toAddress string) (bool, error)
// }