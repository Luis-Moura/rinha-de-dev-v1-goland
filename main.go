package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Evento struct {
	ID                  int    `json:"id"`
	Nome                string `json:"nome"`
	IngressosDisponiveis int    `json:"ingressos_disponiveis"`
}

var db *pgxpool.Pool

func main() {
	connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s",
		getEnv("DB_USER", "admin"),
		getEnv("DB_PASS", "123"),
		getEnv("DB_HOST", "localhost"),
		getEnv("DB_PORT", "5432"),
		getEnv("DB_NAME", "rinha"),
	)

	config, err := pgxpool.ParseConfig(connStr)
	if err != nil {
    fmt.Fprintf(os.Stderr, "Erro na configuração do banco: %v\n", err)
    os.Exit(1)}

	config.MaxConns = 15 
	config.MinConns = 0
	config.MaxConnIdleTime = time.Minute


	db, err = pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		os.Exit(1)
	}

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()

	r.GET("/eventos", listarEventos)
	r.POST("/reservas", realizarReserva)

	r.Run(":8080")
}

func listarEventos(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	rows, err := db.Query(ctx, "SELECT id, nome, ingressos_disponiveis FROM eventos")
	if err != nil {
		c.Status(500)
		return
	}
	defer rows.Close()

	eventos := make([]Evento, 0, 1)
	for rows.Next() {
		var e Evento
		if err := rows.Scan(&e.ID, &e.Nome, &e.IngressosDisponiveis); err == nil {
			eventos = append(eventos, e)
		}
	}
	c.JSON(200, eventos)
}

func realizarReserva(c *gin.Context) {
	var req struct {
		EventoID  int `json:"evento_id"`
		UsuarioID int `json:"usuario_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Status(400)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	tx, err := db.Begin(ctx)
	if err != nil {
		c.Status(500)
		return
	}
	defer tx.Rollback(ctx)

	var id int
	err = tx.QueryRow(ctx, `
		UPDATE eventos 
		SET ingressos_disponiveis = ingressos_disponiveis - 1 
		WHERE id = $1 AND ingressos_disponiveis > 0 
		RETURNING id`, req.EventoID).Scan(&id)

	if err != nil {
    if err == pgx.ErrNoRows {
        c.Status(422)
    } else {
        c.Status(500)
    }
    return
}

	_, err = tx.Exec(ctx, "INSERT INTO reservas (evento_id, usuario_id) VALUES ($1, $2)", req.EventoID, req.UsuarioID)
	if err != nil {
		c.Status(500)
		return
	}

	if err := tx.Commit(ctx); err != nil {
		c.Status(500)
		return
	}

	c.Status(201)
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok { return v }
	return fallback
}