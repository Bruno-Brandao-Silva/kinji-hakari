package bot

import (
	"log/slog"
	"time"

	disgobot "github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/snowflake/v2"
)

// OnVoiceLeave cleans up when the bot is disconnected and leaves when it is left alone.
func (b *Bot) OnVoiceLeave(e *events.GuildVoiceLeave) {
	client := e.Client()
	guildID := e.VoiceState.GuildID
	oldChannel := e.OldVoiceState.ChannelID

	if e.VoiceState.UserID == client.ID() {
		b.handleBotDisconnected(client, guildID, oldChannel)
		return
	}
	b.leaveIfAlone(client, guildID, oldChannel)
}

// OnVoiceMove follows the bot when it is moved and checks the channel a user left.
func (b *Bot) OnVoiceMove(e *events.GuildVoiceMove) {
	client := e.Client()
	guildID := e.VoiceState.GuildID

	if e.VoiceState.UserID == client.ID() {
		b.followBotMove(guildID, e.VoiceState.ChannelID)
		return
	}
	b.leaveIfAlone(client, guildID, e.OldVoiceState.ChannelID)
}

func (b *Bot) handleBotDisconnected(client *disgobot.Client, guildID snowflake.ID, oldChannel *snowflake.ID) {
	if !b.sessionInChannel(guildID, oldChannel) {
		return
	}
	slog.Info("Bot desconectado do canal de voz", "guild_id", guildID)
	b.leave(client, guildID)
}

func (b *Bot) followBotMove(guildID snowflake.ID, channelID *snowflake.ID) {
	if channelID == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if s, ok := b.sessions[guildID]; ok {
		s.channelID = *channelID
	}
}

func (b *Bot) leaveIfAlone(client *disgobot.Client, guildID snowflake.ID, channelID *snowflake.ID) {
	if !b.sessionInChannel(guildID, channelID) || !isAlone(client, guildID, *channelID) {
		return
	}
	slog.Info("Bot sozinho no canal, agendando saída...", "guild_id", guildID)
	time.AfterFunc(aloneLeaveDelay, func() {
		if b.sessionInChannel(guildID, channelID) && isAlone(client, guildID, *channelID) {
			slog.Info("Bot ainda sozinho, saindo.", "guild_id", guildID)
			b.leave(client, guildID)
		}
	})
}

func isAlone(client *disgobot.Client, guildID, channelID snowflake.ID) bool {
	for state := range client.Caches.VoiceStates(guildID) {
		if state.UserID != client.ID() && state.ChannelID != nil && *state.ChannelID == channelID {
			return false
		}
	}
	return true
}
