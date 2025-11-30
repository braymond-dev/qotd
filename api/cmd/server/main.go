package main

import (
	"context"
	"log"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"

	"qotd/api/internal/db"
	"qotd/api/internal/httpserver"
	"qotd/api/internal/llm"
	"qotd/api/internal/service"
)

type Config struct {
	DBURL      string
	Addr       string
	CronKey    string
	OpenAIKey  string
	EmbedModel string
	GradeModel string
}

func LoadConfig() Config {
	return Config{
		DBURL:      getenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/qotd?sslmode=disable"),
		Addr:       getenv("ADDR", ":8080"),
		CronKey:    os.Getenv("CRON_KEY"),
		OpenAIKey:  os.Getenv("OPENAI_API_KEY"),
		EmbedModel: getenv("OPENAI_EMBED_MODEL", "text-embedding-3-small"),
		GradeModel: getenv("OPENAI_GRADE_MODEL", "gpt-4o-mini"),
	}
}

func main() {
	cfg := LoadConfig()
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.DBURL)
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}
	defer pool.Close()

	repo := db.NewRepository(pool)
	if cfg.OpenAIKey == "" {
		log.Println("warning: OPENAI_API_KEY not set; LLM calls will fail at runtime")
	}
	logger := log.Default()
	svc := service.NewQuestionService(
		repo,
		llm.NewGrader(cfg.OpenAIKey, cfg.GradeModel),
		llm.NewEmbedder(cfg.OpenAIKey, cfg.EmbedModel),
		llm.NewGenerator(cfg.OpenAIKey, cfg.GradeModel),
		logger,
	)
	server := httpserver.New(svc, cfg.CronKey)
	if err := server.Start(cfg.Addr); err != nil {
		log.Fatal(err)
	}
}

func getenv(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
