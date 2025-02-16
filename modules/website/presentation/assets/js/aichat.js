class AiChatWidget extends HTMLElement {
	constructor() {
		super();
		this.attachShadow({ mode: 'open' });
	}

	async connectedCallback() {
		const title = this.getAttribute('title') || 'AI Chat';
		const description = this.getAttribute('description') || 'A simple AI chat widget';
		const params = new URLSearchParams({ title, description }).toString();
		const response = await fetch('/website/ai-chat/payload?' + params);
		const html = await response.text();

		this.shadowRoot.innerHTML = html;

		const textarea = this.shadowRoot.getElementById("message-textarea");
		const sendButton = this.shadowRoot.getElementById("send-button");

		textarea.addEventListener("input", () => {
			sendButton.disabled = !textarea.value;
		});

		sendButton.addEventListener("click", async () => {
			console.log("Send button clicked");
		});
	}
}

class ChatStarter extends HTMLElement {
	constructor() {
		super();
		this.attachShadow({ mode: 'open' });
	}

	async connectedCallback() {
		this.shadowRoot.innerHTML = `
		<div class="faq-item">
			<slot></slot>
		</div>
		`;
	}
}

customElements.define('ai-chat-widget', AiChatWidget);
customElements.define('chat-starter', ChatStarter);

