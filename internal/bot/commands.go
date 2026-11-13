package bot

import (
	"fmt"
	"log/slog"
	"os/exec"
	"runtime"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

const hakariGifURL = "https://media.tenor.com/Rpk3q-OLFeYAAAAC/hakari-dance-hakari.gif"

// Commands returns the slash commands the bot registers globally.
func Commands() []discord.ApplicationCommandCreate {
	minVolume, maxVolume := 0, 100
	return []discord.ApplicationCommandCreate{
		discord.SlashCommandCreate{
			Name:        "jackpot",
			Description: "Kinji Hakari expande seu domínio.",
			Options: []discord.ApplicationCommandOption{
				discord.ApplicationCommandOptionFloat{
					Name:        "quantas-vezes",
					Description: "Quantas vezes repetir? (Vazio = Infinito)",
				},
				discord.ApplicationCommandOptionInt{
					Name:        "volume",
					Description: "Volume da música (0-200, Padrão: 100)",
					MinValue:    &minVolume,
					MaxValue:    &maxVolume,
				},
			},
		},
		discord.SlashCommandCreate{
			Name:        "status",
			Description: "Verifica o status do bot e dependências.",
		},
		discord.SlashCommandCreate{
			Name:        "leave",
			Description: "Kinji Hakari libera seu domínio.",
			Options: []discord.ApplicationCommandOption{
				discord.ApplicationCommandOptionBool{
					Name:        "apos-musica",
					Description: "Sair apenas após o término da batida atual?",
				},
			},
		},
	}
}

// OnCommand dispatches slash command interactions.
func (b *Bot) OnCommand(e *events.ApplicationCommandInteractionCreate) {
	data := e.SlashCommandInteractionData()
	log := slog.With("command", data.CommandName(), "user_id", e.User().ID, "guild_id", e.GuildID())
	log.Info("Comando recebido")

	switch data.CommandName() {
	case "jackpot":
		b.handleJackpot(e, data, log)
	case "leave":
		b.handleLeave(e, data, log)
	case "status":
		b.handleStatus(e, log)
	}
}

func (b *Bot) handleJackpot(e *events.ApplicationCommandInteractionCreate, data discord.SlashCommandInteractionData, log *slog.Logger) {
	guildID := e.GuildID()
	if guildID == nil {
		reply(e, log, "Use este comando em um servidor.")
		return
	}
	voiceState, ok := e.Client().Caches.VoiceState(*guildID, e.User().ID)
	if !ok || voiceState.ChannelID == nil {
		reply(e, log, "Você precisa estar em um canal de voz!")
		return
	}

	loops, volume := 0, 100
	if value, ok := data.OptFloat("quantas-vezes"); ok {
		loops = int(value)
	}
	if value, ok := data.OptInt("volume"); ok {
		volume = value
	}

	embed := discord.Embed{
		Title:       "Kinji Hakari expande seu domínio",
		Description: "JACKPOT!",
		Color:       0x7efba6,
		Image:       &discord.EmbedResource{URL: hakariGifURL},
	}
	if err := e.CreateMessage(discord.MessageCreate{Embeds: []discord.Embed{embed}}); err != nil {
		log.Error("Erro ao responder interação", "error", err)
		return
	}

	log.Info("Iniciando playback", "loops", loops, "volume", volume)
	if err := b.play(e.Client(), *guildID, *voiceState.ChannelID, loops, volume); err != nil {
		log.Error("Erro ao conectar voz", "error", err)
		followup(e, log, "Não consegui entrar no canal de voz. Tente de novo.")
	}
}

func (b *Bot) handleLeave(e *events.ApplicationCommandInteractionCreate, data discord.SlashCommandInteractionData, log *slog.Logger) {
	guildID := e.GuildID()
	if guildID == nil {
		reply(e, log, "Use este comando em um servidor.")
		return
	}

	if afterSong, _ := data.OptBool("apos-musica"); afterSong {
		player := b.currentPlayer(*guildID)
		if player == nil {
			reply(e, log, "Não estou em um canal de voz.")
			return
		}
		player.StopAfterCurrentLoop()
		log.Info("Lazy Exit agendado")
		reply(e, log, "Domínio será liberado após o fim da música.")
		return
	}

	b.leave(e.Client(), *guildID)
	log.Info("Desconectou do canal de voz")
	reply(e, log, "Kinji Hakari liberou seu domínio.")
}

func (b *Bot) handleStatus(e *events.ApplicationCommandInteractionCreate, log *slog.Logger) {
	ffmpegStatus := "❌ Não encontrado"
	if path, err := exec.LookPath("ffmpeg"); err == nil {
		ffmpegStatus = fmt.Sprintf("✅ Instalado (`%s`)", path)
	}

	inline := true
	embed := discord.Embed{
		Title: "Status do Sistema",
		Color: 0x3498db,
		Fields: []discord.EmbedField{
			{Name: "Latência API", Value: fmt.Sprintf("%d ms", e.Client().Gateway.Latency().Milliseconds()), Inline: &inline},
			{Name: "FFmpeg", Value: ffmpegStatus, Inline: &inline},
			{Name: "Goroutines", Value: fmt.Sprintf("%d", runtime.NumGoroutine()), Inline: &inline},
		},
	}
	if err := e.CreateMessage(discord.MessageCreate{Embeds: []discord.Embed{embed}}); err != nil {
		log.Error("Erro ao responder interação", "error", err)
	}
}

func reply(e *events.ApplicationCommandInteractionCreate, log *slog.Logger, content string) {
	if err := e.CreateMessage(discord.MessageCreate{Content: content}); err != nil {
		log.Error("Erro ao responder interação", "error", err)
	}
}

func followup(e *events.ApplicationCommandInteractionCreate, log *slog.Logger, content string) {
	message := discord.MessageCreate{Content: content}
	if _, err := e.Client().Rest.CreateFollowupMessage(e.ApplicationID(), e.Token(), message); err != nil {
		log.Error("Erro ao enviar followup", "error", err)
	}
}
