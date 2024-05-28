import 'dotenv/config'
import { Client, Events, GatewayIntentBits, REST, Routes, ChatInputCommandInteraction } from 'discord.js';
import { JackpotCommand, JackpotCommandJSON } from './commands/Jackpot';
import { LeaveCommand, LeaveCommandJSON } from './commands/Leave';
import { VoiceConnectionManager } from './models/VoiceConnectionManager';


const { TOKEN, CLIENT_ID } = process.env;

if (!TOKEN || !CLIENT_ID) {
    throw new Error('Variáveis de ambiente não definidas.');
}

const Definer = () => {
    try {
        const rest = new REST({ version: '10' }).setToken(TOKEN);
        const commands = [
            JackpotCommandJSON,
            LeaveCommandJSON
        ];
        rest.put(Routes.applicationCommands(CLIENT_ID), { body: commands }).then(() => {
            console.log('Successfully reloaded application (/) commands.');
        });
    } catch (error) {
        console.error(error);
    }
}


const client = new Client({
    intents: [
        GatewayIntentBits.Guilds,
        GatewayIntentBits.GuildVoiceStates,
    ]
});


client.once(Events.ClientReady, () => {
    console.log(`Bot está logado como ${client.user?.tag}`);
    client.user?.setAvatar('./hakari-dance-hakari.gif')
        .then(user => console.log(`Changed profile picture for ${user.tag}`))
        .catch(console.error);
});

client.on(Events.InteractionCreate, async interaction => {
    if (!interaction.isCommand()) return;

    const { commandName } = interaction;
    if (!(interaction instanceof ChatInputCommandInteraction)) return;

    if (commandName === 'jackpot') {
        JackpotCommand(interaction);
    } else if (commandName === 'leave') {
        LeaveCommand(interaction);
    }

});

client.on(Events.VoiceStateUpdate, (oldState, newState) => {
    if (oldState.id === client.user?.id) {
        const manager = VoiceConnectionManager.getConnection(oldState.guild.id);
        if (manager && (!newState.channelId || newState.channelId !== manager.channelId)) {
            VoiceConnectionManager.deleteConnection(oldState.guild.id);
        }
    }
});

Definer();
client.login(TOKEN);