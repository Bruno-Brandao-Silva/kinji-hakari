import { AudioPlayer, VoiceConnection, VoiceConnectionStatus } from "@discordjs/voice";

export type VoiceConnectionType = {
    channelId: string;
    intervalId: NodeJS.Timeout;
    connection: VoiceConnection;
    audioPlayer: AudioPlayer;
}

export class VoiceConnectionManager {
    static connections: Map<string, VoiceConnectionType> = new Map();

    static setConnection(serverId: string, channelId: string, intervalId: NodeJS.Timeout, connection: VoiceConnection, audioPlayer: AudioPlayer) {
        this.connections.set(serverId, { channelId, intervalId, connection, audioPlayer });
    }

    static getConnection(serverId: string) {
        return this.connections.get(serverId);
    }

    static deleteConnection(serverId: string) {
        const manager = this.connections.get(serverId);
        if (manager) {
            clearInterval(manager.intervalId);
            manager.connection.state.status !== VoiceConnectionStatus.Destroyed && manager.connection.destroy();
        }
        this.connections.delete(serverId);
    }
}