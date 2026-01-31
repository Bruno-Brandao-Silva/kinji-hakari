# Kinji Hakari Bot 🎰

> *"Always bet on Hakari."*

Um bot de Discord focado em tocar a música tema do Kinji Hakari (**Tuca Donka**) em loop, simulando a expansão de domínio "Idle Death Gamble".

## 🚀 Funcionalidades

- **Jackpot Musique**: Toca "Tuca Donka" em loop no canal de voz.
- **Visuals**: Exibe o GIF da dança do Hakari.
- **Robustez**: Reconexão automática em caso de queda de voz.
- **Controle Total**: Ajuste de volume e loops.

## 🛠️ Comandos

- `/jackpot [quantas-vezes] [volume]`
  - `quantas-vezes`: Número de repetições (Vazio = Infinito).
  - `volume`: Volume do áudio de 0 a 200 (Padrão: 100).
- `/leave [apos-musica]`: Sai do canal de voz (imediatamente ou após terminar a música atual).
- `/status`: Verifica latência da API e status do FFmpeg.

## 📦 Como Rodar

### Pré-requisitos
- **Token do Discord**: Crie um bot no [Discord Developer Portal](https://discord.com/developers/applications).
- **FFmpeg**: Necessário para processamento de áudio.

### Usando Docker (Recomendado)

```bash
# 1. Construir a imagem
docker build -t hakari-bot .

# 2. Rodar o container
docker run -d --name hakari -e TOKEN=seu_token_aqui hakari-bot
```

### Rodando Manualmente (Go)

1. Instale o FFmpeg:
   - Linux: `sudo apt install ffmpeg`
   - Windows: Baixe e adicione ao PATH.
2. Clone o repositório.
3. Crie um arquivo `.env` com seu token (use `.env.template` como base).
4. Execute:
   ```bash
   go run main.go
   ```

## 🔧 Estrutura do Projeto

- `main.go`: Ponto de entrada.
- `internal/bot`: Lógica dos comandos Slash.
- `internal/voice`: Gerenciador de voz (com fix para Race Conditions).
- `Dockerfile`: Configuração para deploy.
