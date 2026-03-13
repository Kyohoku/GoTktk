package account

import (
	"context"
	"fmt"
	"gotik/internal/auth"
	rediscache "gotik/internal/middleware/redis"
	"log"
	"time"

	"golang.org/x/crypto/bcrypt"
)

//业务逻辑

type AccountService struct {
	accountRepository *AccountRepository
	cache             *rediscache.Client
}

func NewAccountService(accountRepository *AccountRepository, cache *rediscache.Client) *AccountService {
	return &AccountService{accountRepository: accountRepository, cache: cache}
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

	if as.cache != nil { //将获得的 token 写入 redis
		cacheCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
		defer cancel()

		if err := as.cache.SetBytes(cacheCtx, fmt.Sprintf("account:%d", account.ID), []byte(token), 24*time.Hour); err != nil {
			log.Printf("failed to set cache: %v", err)
		} else {
			//log.Printf("token cached: key=%s token=%s", account.ID, token)  //test 查看当前token是否成功写入redis
		}
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

func (as *AccountService) Logout(ctx context.Context, accountID uint) error {
	account, err := as.FindByID(ctx, accountID)
	if err != nil {
		return err
	}
	if account.Token == "" {
		return nil
	}
	if as.cache != nil { //delete the token in cache
		cacheCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
		defer cancel()

		if err := as.cache.Del(cacheCtx, fmt.Sprintf("account:%d", account.ID)); err != nil {
			log.Printf("failed to del cache: %v", err)
		}
	}
	return as.accountRepository.Logout(ctx, account.ID)
}
