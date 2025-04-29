class AiChatWidget extends HTMLElement {
	constructor() {
		super();
		this.attachShadow({ mode: 'open' });
		this.threadId = null;
	}
	
	updatePhoneInputVisibility() {
		const phoneInput = this.shadowRoot.getElementById("phone-input");
		if (phoneInput) {
			phoneInput.style.display = this.threadId ? "none" : "block";
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
		
		// Clear existing messages
		chatMessages.innerHTML = '';
		
		// Add each message to the chat
		messages.forEach(msg => {
			const messageEl = document.createElement('div');
			messageEl.className = `message ${msg.role === 'user' ? 'user-message' : 'assistant-message'}`;
			messageEl.textContent = msg.message;
			chatMessages.appendChild(messageEl);
		});
		
		// Scroll to the bottom
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
		
		// Show phone input only when thread is null
		this.updatePhoneInputVisibility();

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

			// Add user message to chat immediately for better UX
			const chatMessages = this.shadowRoot.getElementById("chat-messages");
			if (chatMessages) {
				const userMessageEl = document.createElement('div');
				userMessageEl.className = 'message user-message';
				userMessageEl.textContent = message;
				chatMessages.appendChild(userMessageEl);
				chatMessages.scrollTop = chatMessages.scrollHeight;
			}

			// Clear textarea right away
			const messageText = message; // Store message before clearing textarea
			textarea.value = '';
			sendButton.disabled = true;

			try {
				let url;
				let requestBody;
				
				// Get phone number from input field
				const phoneInput = this.shadowRoot.getElementById("phone-input");
				const phoneNumber = phoneInput ? phoneInput.value.trim() : "";
				
				// If we already have a thread ID, send the message to that thread
				if (this.threadId) {
					const baseUrl = this.getAttribute('endpoint') || '/api/chat';
					url = baseUrl.replace('/message', '') + '/messages/' + this.threadId;
					requestBody = { 
						message: messageText,
						phone: phoneNumber  // Include phone even for existing threads
					};
				} else {
					// First message - create a new thread
					if (!phoneNumber) {
						throw new Error('Phone number is required for the first message');
					}
					url = this.getAttribute('endpoint') || '/api/chat';
					requestBody = { 
						message: messageText,
						phone: phoneNumber  // Include phone for new thread
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

				// If this is the first message, store the thread ID
				if (!this.threadId && data.thread_id) {
					this.threadId = data.thread_id;
					// Update phone input visibility when thread ID changes
					this.updatePhoneInputVisibility();
				}
				
				// Fetch messages for this thread
				await this.fetchMessages();

			} catch (error) {
				console.error('Error sending message:', error);
				// Show error in chat
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