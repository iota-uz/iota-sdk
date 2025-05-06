class AiChatWidget extends HTMLElement {
	constructor() {
		super();
		this.attachShadow({ mode: 'open' });
		this.threadId = null;
		this.localStorageKey = 'chat-threadId';
	}

	saveThreadIdToLocalStorage() {
		if (this.threadId) {
			localStorage.setItem(this.localStorageKey, this.threadId);
		}
	}

	loadThreadIdFromLocalStorage() {
		const threadId = localStorage.getItem(this.localStorageKey);
		if (threadId) {
			this.threadId = threadId;
			return true;
		}
		return false;
	}

	updateInputAndFaqVisibility() {
		const phoneInput = this.shadowRoot.getElementById("phone-input");
		if (phoneInput) {
			phoneInput.style.display = this.threadId ? "none" : "block";
		}
		
		const faqContainer = this.shadowRoot.querySelector(".faq");
		if (faqContainer) {
			faqContainer.style.display = this.threadId ? "none" : "flex";
		}
	}

	async fetchMessages() {
		if (!this.threadId) return;

		try {
			const baseUrl = this.getAttribute('endpoint') || '/api/chat';
			const url = baseUrl.replace('/message', '') + '/messages/' + this.threadId;

			const response = await fetch(url, {
				method: 'GET',
				headers: {
					'Content-Type': 'application/json',
				}
			});

			if (!response.ok) {
				if (response.status === 404) {
					localStorage.removeItem(this.localStorageKey);
					this.threadId = null;
					this.updateInputAndFaqVisibility();
					return;
				}
				throw new Error('Network response was not ok');
			}

			const data = await response.json();
			this.displayMessages(data.messages);

		} catch (error) {
			console.error('Error fetching messages:', error);
		}
	}

	displayMessages(messages) {
		const chatMessages = this.shadowRoot.getElementById("chat-messages");
		if (!chatMessages) return;

		chatMessages.innerHTML = '';

		messages.forEach(msg => {
			const messageEl = document.createElement('div');
			messageEl.className = `message ${msg.role === 'user' ? 'user-message' : 'assistant-message'}`;
			messageEl.textContent = msg.message;
			chatMessages.appendChild(messageEl);
		});

		chatMessages.scrollTop = chatMessages.scrollHeight;
	}

	async connectedCallback() {
		const title = this.getAttribute('title') || 'AI Chat';
		const description = this.getAttribute('description') || 'A simple AI chat widget';
		const params = new URLSearchParams({ title, description }).toString();
		const response = await fetch('/website/ai-chat/payload?' + params);
		const html = await response.text();

		this.shadowRoot.innerHTML = html;

		const header = this.shadowRoot.getElementById("header");
		const textarea = this.shadowRoot.getElementById("message-textarea");
		const phoneInput = this.shadowRoot.getElementById("phone-input");
		const sendButton = this.shadowRoot.getElementById("send-button");

		const hasExistingThread = this.loadThreadIdFromLocalStorage();

		this.updateInputAndFaqVisibility();

		if (hasExistingThread) {
			this.fetchMessages();
		}

		header.addEventListener("click", () => {
			const chatBody = this.shadowRoot.getElementById("body");
			const caretDown = this.shadowRoot.getElementById("caret-down");
			chatBody.classList.toggle("hidden");
			caretDown.classList.toggle("rotate-180");
		});

		textarea.addEventListener("input", () => {
			sendButton.disabled = !textarea.value;
		});

		sendButton.addEventListener("click", async () => {
			const message = textarea.value.trim();
			if (!message) return;

			const chatMessages = this.shadowRoot.getElementById("chat-messages");
			if (chatMessages) {
				const userMessageEl = document.createElement('div');
				userMessageEl.className = 'message user-message';
				userMessageEl.textContent = message;
				chatMessages.appendChild(userMessageEl);
				chatMessages.scrollTop = chatMessages.scrollHeight;
			}

			const messageText = message;
			textarea.value = '';
			sendButton.disabled = true;

			try {
				let url;
				let requestBody;

				const phoneInput = this.shadowRoot.getElementById("phone-input");
				const phoneNumber = phoneInput ? phoneInput.value.trim() : "";

				if (this.threadId) {
					const baseUrl = this.getAttribute('endpoint') || '/api/chat';
					url = baseUrl.replace('/message', '') + '/messages/' + this.threadId;
					requestBody = {
						message: messageText,
						phone: phoneNumber
					};
				} else {
					if (!phoneNumber) {
						throw new Error('Phone number is required for the first message');
					}
					url = this.getAttribute('endpoint') || '/api/chat';
					requestBody = {
						message: messageText,
						phone: phoneNumber
					};
				}

				const response = await fetch(url, {
					method: 'POST',
					headers: {
						'Content-Type': 'application/json',
					},
					body: JSON.stringify(requestBody),
				});

				if (!response.ok) {
					throw new Error('Network response was not ok');
				}

				const data = await response.json();

				if (!this.threadId && data.thread_id) {
					this.threadId = data.thread_id;
					this.saveThreadIdToLocalStorage();
					this.updateInputAndFaqVisibility();
				}

				await this.fetchMessages();

			} catch (error) {
				console.error('Error sending message:', error);
				if (chatMessages) {
					const errorMessageEl = document.createElement('div');
					errorMessageEl.className = 'message assistant-message';
					errorMessageEl.textContent = 'Sorry, there was an error sending your message. Please try again.';
					chatMessages.appendChild(errorMessageEl);
					chatMessages.scrollTop = chatMessages.scrollHeight;
				}
			}
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