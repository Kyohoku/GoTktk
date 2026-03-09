package account

import (
	"context"
	"gotik/internal/auth"

	"golang.org/x/crypto/bcrypt"
)

//业务逻辑

type AccountService struct {
	accountRepository *AccountRepository
}

func NewAccountService(accountRepository *AccountRepository) *AccountService {
	return &AccountService{accountRepository: accountRepository}
}

func (as *AccountService) CreateAccount(ctx context.Context, account *Account) error {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(account.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	account.Password = string(passwordHash)

	if err := as.accountRepository.CreateAccount(ctx, account); err != nil {
		return err
	}

	return nil
}

func (as *AccountService) Login(ctx context.Context, username, password string) (string, error) {
	account, err := as.accountRepository.FindByUsername(ctx, username)
	if err != nil {
		return "", err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(account.Password), []byte(password)); err != nil {
		return "", err
	}

	token, err := auth.GenerateToken(account.ID, account.Username)
	if err != nil {
		return "", err
	}

	if err := as.accountRepository.Login(ctx, account.ID, token); err != nil {
		return "", err
	}

	return token, nil
}

func (as *AccountService) FindByID(ctx context.Context, id uint) (*Account, error) {
	account, err := as.accountRepository.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return account, nil
}

func (as *AccountService) FindByUsername(ctx context.Context, username string) (*Account, error) {
	if account, err := as.accountRepository.FindByUsername(ctx, username); err != nil {
		return nil, err
	} else {
		return account, nil
	}
}
