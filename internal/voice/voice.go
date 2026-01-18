package voice

import (
	"bufio"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"strconv"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"layeh.com/gopus"
)

const (
	channels  = 2
	frameRate = 48000
	frameSize = 960
	maxBytes  = 4000
)

type Session struct {
	GuildID    string
	ChannelID  string
	Connection *discordgo.VoiceConnection
	Cancel     context.CancelFunc
}

type Manager struct {
	sessions map[string]*Session
	mu       sync.RWMutex
}

var GlobalManager = &Manager{
	sessions: make(map[string]*Session),
}

func (m *Manager) GetSession(guildID string) *Session {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sessions[guildID]
}

// Join conecta o bot ao canal de voz de forma segura (sem Deadlock)
func (m *Manager) Join(s *discordgo.Session, guildID, channelID string) (*Session, error) {
	// 1. Verificação rápida com Lock de Leitura
	m.mu.RLock()
	if sess, ok := m.sessions[guildID]; ok {
		m.mu.RUnlock() // Libera lock antes de qualquer operação no Discord
		if sess.ChannelID != channelID {
			slog.Info("Mudando de canal", "guild_id", guildID, "old_channel", sess.ChannelID, "new_channel", channelID)
			// ChangeChannel é rápido, mas idealmente não deve bloquear o manager
			sess.Connection.ChangeChannel(channelID, false, false)
			// Atualizamos o channelID na struct (precisa de Lock de Escrita rápido)
			m.mu.Lock()
			sess.ChannelID = channelID
			m.mu.Unlock()
		}
		return sess, nil
	}
	m.mu.RUnlock()

	// 2. Conecta ao canal de voz (OPERAÇÃO LENTA E BLOQUEANTE)
	slog.Info("Conectando ao canal de voz...", "guild_id", guildID, "channel_id", channelID)
	// IMPORTANTE: Fazemos isso FORA de qualquer Lock do manager para evitar Deadlock
	// com os Event Handlers que precisam ler o manager.
	vc, err := s.ChannelVoiceJoin(guildID, channelID, false, true)
	if err != nil {
		return nil, err
	}

	// 3. Registra a sessão com Lock de Escrita
	m.mu.Lock()
	defer m.mu.Unlock()

	// Verifica se outra goroutine não criou a sessão enquanto conectávamos
	if sess, ok := m.sessions[guildID]; ok {
		vc.Disconnect() // Fecha a conexão duplicada
		return sess, nil
	}

	sess := &Session{
		GuildID:    guildID,
		ChannelID:  channelID,
		Connection: vc,
	}
	m.sessions[guildID] = sess
	return sess, nil
}

func (m *Manager) Leave(guildID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if sess, ok := m.sessions[guildID]; ok {
		if sess.Cancel != nil {
			sess.Cancel()
		}
		// Disconnect pode demorar um pouco, mas no Leave é aceitável segurar o lock
		// para garantir consistência de estado imediata.
		sess.Connection.Disconnect()
		delete(m.sessions, guildID)
		slog.Info("Sessão de voz encerrada", "guild_id", guildID)
	}
}

func (sess *Session) PlayLoop(filePath string, loops int) {
	if sess.Cancel != nil {
		sess.Cancel()
	}

	ctx, cancel := context.WithCancel(context.Background())
	sess.Cancel = cancel

	go func() {
		log := slog.With("guild_id", sess.GuildID)
		defer func() {
			// Só desconecta se NÃO foi cancelado (cancelado significa que outra música começou ou comando stop foi dado mas queremos controlar o leave manualmente)
			// Na verdade, se foi cancelado por "substituição", não queremos sair.
			// Se foi cancelado por "leave", o manager já tratou.
			// Vamos verificar se o erro do contexto é Canceled.
			if ctx.Err() == context.Canceled {
				// Contexto cancelado explicitamente (provavelmente nova música tocando).
				// Não saímos do canal.
				log.Info("Playback cancelado (substituição)")
				return
			}
			
			log.Info("Playback finalizado, saindo do canal em 1s...")
			time.Sleep(1 * time.Second)
			GlobalManager.Leave(sess.GuildID)
		}()

		// 1. Aguarda conexão estar PRONTA (Ready) com Timeout
		// O handshake de voz (v4/v5) pode demorar devido ao IP Discovery e negociação de criptografia.
		timeout := time.After(10 * time.Second)
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		ready := false
		for !ready {
			select {
			case <-ctx.Done():
				return
			case <-timeout:
				log.Warn("Timeout aguardando Voice Connection Ready", "ready", sess.Connection.Ready)
				return
			case <-ticker.C:
				if sess.Connection.Ready {
					ready = true
				}
			}
		}

		// 2. Define falando como TRUE
		sess.Connection.Speaking(true)
		defer func() {
			if sess.Connection.Ready {
				sess.Connection.Speaking(false)
			}
		}()

		// 3. Envia frames de silêncio para "aquecer" a conexão UDP e o SSRC
		// Isso é CRÍTICO para evitar "broken pipe" ou desconexão imediata em gateways novos.
		if err := sendSilence(sess.Connection); err != nil {
			log.Warn("Erro enviando silêncio", "error", err)
		}

		loopCount := 0
		infinite := loops <= 0

		for {
			select {
			case <-ctx.Done():
				return
			default:
				if !infinite && loopCount >= loops {
					return
				}

				if err := playAudioFile(ctx, sess.Connection, filePath); err != nil {
					log.Error("Erro tocando áudio", "error", err, "loop", loopCount)
					// Se ocorrer erro de conexão (ex: broken pipe), encerra
					return
				}
				loopCount++
				time.Sleep(100 * time.Millisecond)
			}
		}
	}()
}

// sendSilence envia alguns pacotes de silêncio para estabelecer a prioridade RTP
func sendSilence(vc *discordgo.VoiceConnection) error {
	// 5 frames de silêncio (20ms cada) = 100ms de pre-roll
	for i := 0; i < 5; i++ {
		// Frame Opus de silêncio padrão (FC F8 F8...) ou apenas dados vazios que o encoder tratará?
		// A maneira mais segura com discordgo é enviar o buffer de silêncio PCM codificado.
		// Mas podemos enviar o frame Opus de silêncio manualmente se soubermos.
		// O frame opus para silêncio mono/stereo tocável é tipicamente 3 bytes: 0xF8, 0xFF, 0xFE
		
		silenceFrame := []byte{0xF8, 0xFF, 0xFE}
		
		if !vc.Ready || vc.OpusSend == nil {
			return fmt.Errorf("voice connection not ready for silence")
		}
		
		vc.OpusSend <- silenceFrame
		time.Sleep(20 * time.Millisecond)
	}
	return nil
}

func playAudioFile(ctx context.Context, vc *discordgo.VoiceConnection, filepath string) error {
	run := exec.CommandContext(ctx, "ffmpeg", "-i", filepath, "-f", "s16le", "-ar", strconv.Itoa(frameRate), "-ac", strconv.Itoa(channels), "pipe:1")
	ffmpegOut, err := run.StdoutPipe()
	if err != nil {
		return err
	}

	if err := run.Start(); err != nil {
		return err
	}
	defer run.Wait()

	ffmpegbuf := bufio.NewReaderSize(ffmpegOut, 16384)
	encoder, err := gopus.NewEncoder(frameRate, channels, gopus.Audio)
	if err != nil {
		return fmt.Errorf("falha encoder: %v", err)
	}

	pcmBuf := make([]int16, frameSize*channels)
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			err := binary.Read(ffmpegbuf, binary.LittleEndian, &pcmBuf)
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				return nil
			}
			if err != nil {
				return err
			}

			opusData, err := encoder.Encode(pcmBuf, frameSize, maxBytes)
			if err != nil {
				continue
			}

			if !vc.Ready || vc.OpusSend == nil {
				// Tenta esperar um pouco ou falha
				return fmt.Errorf("conexão não pronta")
			}

			vc.OpusSend <- opusData
		}
	}
}
