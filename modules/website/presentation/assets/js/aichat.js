class AiChatWidget extends HTMLElement {
	constructor() {
		super();
		this.attachShadow({ mode: 'open' });
	}

	async connectedCallback() {
		const response = await fetch('/ai-chat');
		const html = await response.text();

		this.shadowRoot.innerHTML = html;
	}
}

customElements.define('ai-chat-widget', AiChatWidget);

