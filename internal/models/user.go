package models

// User é uma conta de acesso ao estúdio. is_admin vive na tabela users (o
// middleware RequireAdmin lê a coluna direto), não no modelo.
type User struct {
	ID           int64
	Email        string
	PasswordHash string
}
