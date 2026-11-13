package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/disgoorg/disgo"
	disgobot "github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/cache"
	"github.com/disgoorg/disgo/gateway"
	"github.com/disgoorg/disgo/voice"
	"github.com/joho/godotenv"
	"github.com/thomas-vilte/dave-go/session"

	"hakari-bot/internal/audio"
	"hakari-bot/internal/bot"
	"hakari-bot/internal/logger"
)

const (
	audioPath       = "./tuca-donka.mp3"
	shutdownTimeout = 10 * time.Second
)

func main() {
	logger.Init()
	if err := run(); err != nil {
		slog.Error("Erro fatal", "error", err)
		os.Exit(1)
	}
}

func run() error {
	if err := godotenv.Load(); err != nil {
		slog.Warn("Arquivo .env não encontrado, usando vars do sistema.")
	}
	token := os.Getenv("TOKEN")
	if token == "" {
		return errors.New("TOKEN não definido")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	track, err := audio.DecodeFile(ctx, audioPath)
	if err != nil {
		return fmt.Errorf("carregar áudio: %w", err)
	}
	slog.Info("Áudio carregado na memória com sucesso!")

	client, err := newClient(token, bot.New(track))
	if err != nil {
		return fmt.Errorf("criar cliente: %w", err)
	}
	defer closeClient(client)

	slog.Info("Registrando comandos...")
	if _, err := client.Rest.SetGlobalCommands(client.ApplicationID, bot.Commands()); err != nil {
		return fmt.Errorf("registrar comandos: %w", err)
	}
	if err := client.OpenGateway(ctx); err != nil {
		return fmt.Errorf("abrir gateway: %w", err)
	}

	<-ctx.Done()
	slog.Info("Desligando...")
	return nil
}

func newClient(token string, hakari *bot.Bot) (*disgobot.Client, error) {
	return disgo.New(token,
		disgobot.WithLogger(slog.Default()),
		disgobot.WithGatewayConfigOpts(gateway.WithIntents(gateway.IntentGuilds, gateway.IntentGuildVoiceStates)),
		disgobot.WithCacheConfigOpts(cache.WithCaches(cache.FlagVoiceStates)),
		// Handlers block on voice handshakes, which need further gateway events to arrive.
		disgobot.WithEventManagerConfigOpts(disgobot.WithAsyncEventsEnabled()),
		disgobot.WithEventListenerFunc(hakari.OnReady),
		disgobot.WithEventListenerFunc(hakari.OnCommand),
		disgobot.WithEventListenerFunc(hakari.OnVoiceLeave),
		disgobot.WithEventListenerFunc(hakari.OnVoiceMove),
		disgobot.WithVoiceManagerConfigOpts(voice.WithDaveSessionCreateFunc(session.CreateFunc())),
	)
}

func closeClient(client *disgobot.Client) {
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	client.Close(ctx)
}
