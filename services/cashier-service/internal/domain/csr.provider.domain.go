// domain/provider.go
package domain

type Provider interface {
    Name() string
    Deposit(req DepositRequest) (DepositResponse, error)
    Withdraw(req WithdrawRequest) (WithdrawResponse, error)
}
