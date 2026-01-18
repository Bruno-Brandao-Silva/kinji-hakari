package bot

import (
	"hakari-bot/internal/voice"
	"log/slog"
	"time"

	"github.com/bwmarrin/discordgo"
)

// Definição dos comandos
func GetCommands() []*discordgo.ApplicationCommand {
	return []*discordgo.ApplicationCommand{
		{
			Name:        "jackpot",
			Description: "Kinji Hakari expande seu domínio.",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionNumber,
					Name:        "quantas-vezes",
					Description: "Quantas vezes repetir? (Vazio = Infinito)",
					Required:    false,
				},
			},
		},
		{
			Name:        "leave",
			Description: "Kinji Hakari libera seu domínio.",
		},
	}
}

type Bot struct{}

func NewBot() *Bot {
	return &Bot{}
}

// Handler de Interações (Slash Commands)
func (b *Bot) InteractionHandler(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.Type != discordgo.InteractionApplicationCommand {
		return
	}

	data := i.ApplicationCommandData()
	
	// Logger contextual para a requisição
	log := slog.With(
		"command", data.Name,
		"user_id", i.Member.User.ID,
		"guild_id", i.GuildID,
	)
	
	log.Info("Comando recebido")

	switch data.Name {
	case "jackpot":
		b.handleJackpot(s, i, data, log)
	case "leave":
		b.handleLeave(s, i, log)
	}
}

func (b *Bot) handleJackpot(s *discordgo.Session, i *discordgo.InteractionCreate, data discordgo.ApplicationCommandInteractionData, log *slog.Logger) {
	// Validações iniciais
	guildID := i.GuildID
	if guildID == "" {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{Content: "Use este comando em um servidor."},
		})
		return
	}

	// Encontra o canal de voz do usuário
	userChannelID := ""
	guild, err := s.State.Guild(guildID)
	if err == nil {
		for _, vs := range guild.VoiceStates {
			if vs.UserID == i.Member.User.ID {
				userChannelID = vs.ChannelID
				break
			}
		}
	}

	if userChannelID == "" {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{Content: "Você precisa estar em um canal de voz!"},
		})
		return
	}

	// Verifica loops
	loops := 0
	if len(data.Options) > 0 {
		loops = int(data.Options[0].FloatValue())
	}

	// Responde com Embed
	embed := &discordgo.MessageEmbed{
		Title:       "Kinji Hakari expande seu domínio",
		Description: "JACKPOT!",
		Color:       0x7efba6, // Hex color
		Image: &discordgo.MessageEmbedImage{
			URL: "https://media.tenor.com/Rpk3q-OLFeYAAAAC/hakari-dance-hakari.gif",
		},
	}

	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
		},
	})
	if err != nil {
		log.Error("Erro ao responder interação", "error", err)
		return
	}

	// Lógica de Voz
	sess, err := voice.GlobalManager.Join(s, guildID, userChannelID)
	if err != nil {
		log.Error("Erro ao conectar voz", "error", err)
		return
	}

	// Inicia Playback
	// Certifique-se que o arquivo tuca-donka.mp3 está na raiz
	log.Info("Iniciando playback", "loops", loops, "file", "./tuca-donka.mp3")
	sess.PlayLoop("./tuca-donka.mp3", loops)
}

func (b *Bot) handleLeave(s *discordgo.Session, i *discordgo.InteractionCreate, log *slog.Logger) {
	voice.GlobalManager.Leave(i.GuildID)
	log.Info("Desconectou do canal de voz")

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Kinji Hakari liberou seu domínio.",
		},
	})
}

// VoiceStateUpdateHandler lida com eventos como "Fiquei sozinho no canal"
func (b *Bot) VoiceStateUpdateHandler(s *discordgo.Session, v *discordgo.VoiceStateUpdate) {
	// Se o bot foi desconectado forçadamente ou movido
	if v.UserID == s.State.User.ID {
		if v.ChannelID == "" {
			// Bot desconectou
			slog.Info("Bot desconectado do canal de voz", "guild_id", v.GuildID)
			voice.GlobalManager.Leave(v.GuildID)
		}
		return
	}

	// Lógica para sair se estiver sozinho
	// (Requer consulta à lista de membros do canal, simplificada aqui)
	sess := voice.GlobalManager.GetSession(v.GuildID)
	if sess != nil && sess.ChannelID == v.BeforeUpdate.ChannelID {
		guild, err := s.State.Guild(v.GuildID)
		if err != nil {
			return
		}

		userCount := 0
		for _, vs := range guild.VoiceStates {
			if vs.ChannelID == sess.ChannelID {
				userCount++
			}
		}

		// Se userCount for 1, é só o bot
		if userCount == 1 {
			slog.Info("Bot sozinho no canal, agendando saída...", "guild_id", v.GuildID)
			// Aguarda 5 segundos antes de sair (Debounce simples)
			time.AfterFunc(5*time.Second, func() {
				// Verifica novamente se ainda está sozinho
				// Precisamos de uma nova referência ao guild atualizada
				g, err := s.State.Guild(v.GuildID)
				if err != nil {
					return
				}

				count := 0
				for _, vs := range g.VoiceStates {
					if vs.ChannelID == sess.ChannelID {
						count++
					}
				}

				if count == 1 {
					slog.Info("Bot ainda sozinho, saindo.", "guild_id", v.GuildID)
					voice.GlobalManager.Leave(v.GuildID)
				}
			})
		}
	}
}
