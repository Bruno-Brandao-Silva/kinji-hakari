package main

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"hakari-bot/internal/bot"
	"hakari-bot/internal/logger"
	"hakari-bot/internal/voice"

	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
)

func main() {
	// 1. Inicializa Logger
	logger.Init()

	// 2. Carrega variaveis de ambiente
	if err := godotenv.Load(); err != nil {
		slog.Warn("Arquivo .env não encontrado, usando vars do sistema.")
	}

	// 2.5 Carrega áudio para memória
	if err := voice.LoadAudio("./tuca-donka.mp3"); err != nil {
		slog.Error("Erro fatal ao carregar áudio", "error", err)
		os.Exit(1)
	}
	slog.Info("Áudio carregado na memória com sucesso!")

	token := os.Getenv("TOKEN")
	if token == "" {
		slog.Error("TOKEN não definido.")
		os.Exit(1)
	}

	// 3. Cria sessão do Discord
	s, err := discordgo.New("Bot " + token)
	if err != nil {
		slog.Error("Erro ao criar sessão", "error", err)
		os.Exit(1)
	}

	// 4. Define Intents (ATUALIZAÇÃO CRÍTICA DO DISCORD)
	// GuildVoiceStates é necessário para saber quem está nos canais
	s.Identify.Intents = discordgo.IntentsGuilds | discordgo.IntentsGuildVoiceStates | discordgo.IntentsGuildMessages

	// 5. Injeta handlers
	b := bot.NewBot()
	s.AddHandler(b.InteractionHandler)
	s.AddHandler(b.VoiceStateUpdateHandler)
	s.AddHandler(b.VoiceServerUpdateHandler)

	// 6. Abre conexão
	if err := s.Open(); err != nil {
		slog.Error("Erro ao abrir conexão via socket", "error", err)
		os.Exit(1)
	}
	defer s.Close()

	// 7. Registra Slash Commands
	slog.Info("Registrando comandos...")
	commands := bot.GetCommands()
	for _, cmd := range commands {
		if _, err := s.ApplicationCommandCreate(s.State.User.ID, "", cmd); err != nil {
			slog.Error("Erro ao registrar comando", "command", cmd.Name, "error", err)
			os.Exit(1)
		}
	}

	slog.Info("Bot logado", "user", s.State.User.Username)

	// 8. Graceful Shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	slog.Info("Removendo comandos e desligando...")
	// Opcional: Limpar comandos ao sair para não duplicar em dev
	// for _, cmd := range cmds {
	// 	s.ApplicationCommandDelete(s.State.User.ID, "", cmd.ID)
	// }
}
