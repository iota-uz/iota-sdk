// A fake Telegram Bot API for local demos and manual checks of the Telegram notification channel (#1349).
//
// It answers the Bot API methods the connector calls (getMe, getWebhookInfo, setWebhook, deleteWebhook,
// sendMessage), keeps the webhook the application set, and lets you act as a Telegram user:
//
//   POST /_fake/start   {chat_id, username, first_name, payload, chat_type?}  → delivers `/start <payload>`
//   POST /_fake/block   {chat_id}                                            → sendMessage to the chat gets 403
//   GET  /_fake/sent                                                         → messages the bot sent
//   POST /_fake/reset                                                        → forgets messages and blocks
//
// Usage: node scripts/fake-telegram.mjs <port>, then start cmd/server with TELEGRAM_APIURL=http://127.0.0.1:<port>,
// TELEGRAM_PLATFORMTOKEN=<id>:<35 chars> and EXTRUNTIME_ENABLED=true (the webhook route needs the engine).
import http from 'node:http';

export function startFakeTelegram(port = 0) {
	const state = { webhook: { url: '', secret: '' }, sent: [], blocked: new Set(), updateId: Date.now(), messageId: 1 };
	const server = http.createServer((req, res) => {
		handle(req, res).catch((err) => {
			// An application that went away (the webhook does not answer) must not stop the fake.
			if (!res.headersSent) res.writeHead(502, { 'content-type': 'application/json' });
			res.end(JSON.stringify({ error: String(err && err.message ? err.message : err) }));
		});
	});
	async function handle(req, res) {
		const url = new URL(req.url, 'http://fake');
		const body = await readBody(req);
		const reply = (code, payload) => {
			res.writeHead(code, { 'content-type': 'application/json' });
			res.end(JSON.stringify(payload));
		};
		if (url.pathname.startsWith('/_fake/')) {
			const action = url.pathname.slice('/_fake/'.length);
			if (action === 'sent') return reply(200, state.sent);
			if (action === 'reset') {
				state.sent = [];
				state.blocked.clear();
				return reply(200, { ok: true });
			}
			if (action === 'block') {
				state.blocked.add(String(body.chat_id));
				return reply(200, { ok: true });
			}
			if (action === 'webhook') return reply(200, state.webhook);
			if (action === 'start') {
				if (!state.webhook.url) return reply(409, { error: 'no webhook' });
				const chat = { id: Number(body.chat_id), type: body.chat_type || 'private', username: body.username, first_name: body.first_name };
				const update = {
					update_id: state.updateId++,
					message: {
						message_id: state.messageId++, date: Math.floor(Date.now() / 1000), chat,
						from: { id: Number(body.chat_id), is_bot: false, username: body.username, first_name: body.first_name || 'Test' },
						text: `/start ${body.payload}`, entities: [{ type: 'bot_command', offset: 0, length: 6 }],
					},
				};
				const r = await fetch(state.webhook.url, {
					method: 'POST',
					headers: { 'content-type': 'application/json', 'X-Telegram-Bot-Api-Secret-Token': state.webhook.secret },
					body: JSON.stringify(update),
				});
				return reply(200, { status: r.status });
			}
			return reply(404, { error: 'unknown action' });
		}
		// /bot<token>/<method>
		const match = url.pathname.match(/^\/bot([^/]+)\/([A-Za-z]+)$/);
		if (!match) return reply(404, { ok: false, error_code: 404, description: 'Not Found' });
		const [, token, method] = match;
		const botId = Number(token.split(':')[0]);
		const params = { ...Object.fromEntries(url.searchParams), ...body };
		switch (method) {
			case 'getMe':
				return reply(200, { ok: true, result: { id: botId, is_bot: true, first_name: 'Fake bot', username: 'fake_iota_bot' } });
			case 'getWebhookInfo':
				return reply(200, { ok: true, result: { url: state.webhook.url, pending_update_count: 0 } });
			case 'setWebhook':
				state.webhook = { url: params.url, secret: params.secret_token || '' };
				return reply(200, { ok: true, result: true });
			case 'deleteWebhook':
				state.webhook = { url: '', secret: '' };
				return reply(200, { ok: true, result: true });
			case 'sendMessage': {
				const chat = String(params.chat_id);
				if (state.blocked.has(chat)) {
					return reply(403, { ok: false, error_code: 403, description: 'Forbidden: bot was blocked by the user' });
				}
				const message = { message_id: state.messageId++, date: Math.floor(Date.now() / 1000), chat: { id: Number(chat), type: 'private' }, text: params.text };
				state.sent.push({ chat_id: chat, text: params.text, parse_mode: params.parse_mode || '', reply_markup: params.reply_markup || '' });
				return reply(200, { ok: true, result: message });
			}
			default:
				return reply(200, { ok: true, result: true });
		}
	}
	return new Promise((resolve) => {
		server.listen(port, '127.0.0.1', () => resolve({ server, port: server.address().port, state }));
	});
}

async function readBody(req) {
	const chunks = [];
	for await (const c of req) chunks.push(c);
	const raw = Buffer.concat(chunks).toString('utf8');
	if (!raw) return {};
	const type = req.headers['content-type'] || '';
	if (type.includes('application/json')) {
		try {
			return JSON.parse(raw);
		} catch {
			return {};
		}
	}
	if (type.includes('multipart/form-data')) {
		const boundary = type.split('boundary=')[1];
		const out = {};
		for (const part of raw.split('--' + boundary)) {
			const m = part.match(/name="([^"]+)"\r\n\r\n([\s\S]*?)\r\n$/);
			if (m) out[m[1]] = m[2];
		}
		return out;
	}
	return Object.fromEntries(new URLSearchParams(raw));
}

if (import.meta.url === `file://${process.argv[1]}`) {
	const { port } = await startFakeTelegram(Number(process.argv[2] || 0));
	console.log(`fake telegram on http://127.0.0.1:${port}`);
}
