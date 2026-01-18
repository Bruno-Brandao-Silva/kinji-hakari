package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"hakari-bot/internal/bot"

	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
)

func main() {
	// 1. Carrega variaveis de ambiente
	if err := godotenv.Load(); err != nil {
		log.Println("Aviso: Arquivo .env não encontrado, usando vars do sistema.")
	}

	token := os.Getenv("TOKEN")
	if token == "" {
		log.Fatal("TOKEN não definido.")
	}

	// 2. Cria sessão do Discord
	s, err := discordgo.New("Bot " + token)
	if err != nil {
		log.Fatalf("Erro ao criar sessão: %v", err)
	}

	// 3. Define Intents (ATUALIZAÇÃO CRÍTICA DO DISCORD)
	// GuildVoiceStates é necessário para saber quem está nos canais
	s.Identify.Intents = discordgo.IntentsGuilds | discordgo.IntentsGuildVoiceStates | discordgo.IntentsGuildMessages

	// 4. Injeta handlers
	b := bot.NewBot()
	s.AddHandler(b.InteractionHandler)
	s.AddHandler(b.VoiceStateUpdateHandler)

	// 5. Abre conexão
	if err := s.Open(); err != nil {
		log.Fatalf("Erro ao abrir conexão via socket: %v", err)
	}
	defer s.Close()

	// 6. Registra Slash Commands
	log.Println("Registrando comandos...")
	commands := bot.GetCommands()
	for _, cmd := range commands {
		if _, err := s.ApplicationCommandCreate(s.State.User.ID, "", cmd); err != nil {
			log.Fatalf("Erro ao registrar comando %s: %v", cmd.Name, err)
		}
	}

	log.Printf("Bot logado como %s. Pressione CTRL+C para sair.", s.State.User.Username)

	// 7. Graceful Shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("Removendo comandos e desligando...")
	// Opcional: Limpar comandos ao sair para não duplicar em dev
	// for _, cmd := range cmds {
	// 	s.ApplicationCommandDelete(s.State.User.ID, "", cmd.ID)
	// }
}
