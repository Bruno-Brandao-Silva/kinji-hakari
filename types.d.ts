import { type VoiceConnection, type PlayerSubscription } from '@discordjs/voice';

export type Server = {
    connection: VoiceConnection | null,
    channelId: string | null,
    playerSubscription: PlayerSubscription | null | undefined
}