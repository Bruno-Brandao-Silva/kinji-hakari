package logger

import (
	"log/slog"
	"os"
)

func Init() {
	// Cria um logger JSON que escreve no STDOUT
	// O nível padrão é Info. Podemos parametrizar isso se necessário.
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Define como logger padrão global
	slog.SetDefault(logger)
}
