import { Client, GatewayIntentBits } from 'discord.js';
import dotenv from 'dotenv';
import commands from './commands';
import type { Server } from './types';

dotenv.config();

const { TOKEN, CLIENT_ID, PREFIX } = process.env;
if (!TOKEN || !CLIENT_ID || !PREFIX) {
	throw new Error('Variáveis de ambiente não definidas.');
}


const servers: { [guildId: string]: Server } = {};

const client = new Client({
	intents: [
		GatewayIntentBits.Guilds,
		GatewayIntentBits.GuildMessages,
		GatewayIntentBits.GuildVoiceStates,
		GatewayIntentBits.MessageContent,
	]
});

client.once('ready', () => {
	console.log(`Bot está logado como ${client.user?.tag}`);
});

client.on('messageCreate', async (message) => {
	if (!message.content.startsWith(PREFIX)) return;
	const command = message.content.split(PREFIX)[1]
	if (commands[command] && typeof commands[command] === "function") {
		commands[command](client, message, servers);
	} else {
		console.log(`A função ${command} não existe.`);
	}
});

client.login(TOKEN);
