package account

// 数据表设计
type Account struct {
	ID       uint   `gorm:"primary_key" json:"id"`
	Username string `gorm:"unique" json:"username"`
	Password string `json:"-"`
	Token    string `json:"-"`
}

type CreateAccountRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type FindByIDRequest struct {
	ID uint `json:"id"`
}

type FindByIDResponse struct {
	Username string `json:"username"`
	ID       uint   `json:"id"`
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}
