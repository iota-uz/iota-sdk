package toast

import (
	"fmt"

	"github.com/iota-uz/iota-sdk/pkg/htmx"
)

// Action is the single verb a toast can offer ("Undo", "Retry"). Set Label and
// exactly one of HxPost (the button posts there through HTMX, without a swap) or
// Href (the action is a link). Clicking it runs the action and closes the toast;
// a toast with an action stays at least ActionMinDuration and pauses while it is
// hovered or focused. Send it with htmx.TriggerToastWithAction.
type Action = htmx.ToastAction

// ActionMinDuration is the shortest time, in milliseconds, a toast with an
// action stays before it closes itself.
const ActionMinDuration = 8000

// The class strings below are shared verbatim with the Solid toast
// (web/solid-ui/src/overlays/Toast.tsx); web/solid-ui/vr/templ-parity.spec.ts
// compares both renderings.
const (
	regionClass = "fixed z-[100] pointer-events-none flex flex-col gap-3 top-0 right-0 w-full max-w-sm " +
		"p-4 pt-[max(1rem,env(safe-area-inset-top))] pr-[max(1rem,env(safe-area-inset-right))] " +
		"max-[480px]:top-auto max-[480px]:bottom-[var(--iota-toast-offset-bottom,0px)] max-[480px]:max-w-none max-[480px]:flex-col-reverse " +
		"max-[480px]:pt-3 max-[480px]:pl-[max(0.75rem,env(safe-area-inset-left))] max-[480px]:pr-[max(0.75rem,env(safe-area-inset-right))] max-[480px]:pb-[max(0.75rem,env(safe-area-inset-bottom))]"
	surfaceClass = "flex items-start gap-3 w-full p-4 bg-surface-300 text-100 rounded-lg shadow-lg border border-subtle"
	actionClass  = "flex-shrink-0 self-center -my-1 inline-flex items-center min-h-9 [@media(pointer:coarse)]:min-h-11 px-2 rounded-md text-sm font-semibold text-primary-500 dark:text-primary-400 hover:bg-primary-500/10 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary-500 cursor-pointer"
	closeClass   = "flex-shrink-0 rounded-md text-300 hover:text-100 transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary-500"
)

// toastItemData is the Alpine state of one toast: the auto-dismiss timer
// (default 8 s, 0 keeps the toast, at least ActionMinDuration with an action)
// pauses while the toast is hovered or holds focus.
func toastItemData() string {
	return fmt.Sprintf(`{
	isVisible: false,
	timeout: null,
	remaining: 0,
	startedAt: 0,
	hovered: false,
	focused: false,
	done: false,
	get variant() {
		const variant = notification.variant;
		return ['success', 'error', 'danger', 'warning', 'info'].includes(variant) ? variant : 'info';
	},
	get polite() { return this.variant === 'success' || this.variant === 'info'; },
	get iconClass() {
		return { success: 'text-green-500', error: 'text-red-500', danger: 'text-red-500', warning: 'text-yellow-500', info: 'text-blue-500' }[this.variant];
	},
	get action() {
		const action = notification.action;
		return action && action.label && (action.href || action.hxPost) ? action : null;
	},
	init() {
		const duration = Number.isFinite(notification.duration) ? notification.duration : 8000;
		this.remaining = duration <= 0 ? 0 : (this.action ? Math.max(duration, %d) : duration);
		this.$nextTick(() => {
			this.isVisible = true;
			this.sync();
		});
	},
	sync() {
		if (this.hovered || this.focused) {
			if (this.timeout) this.remaining = Math.max(0, this.remaining - (Date.now() - this.startedAt));
			clearTimeout(this.timeout);
			this.timeout = null;
			return;
		}
		if (this.timeout || this.remaining <= 0 || this.done) return;
		this.startedAt = Date.now();
		this.timeout = setTimeout(() => this.dismiss(), this.remaining);
	},
	runAction() {
		const action = this.action;
		if (!action || this.done) return;
		if (action.hxPost && window.htmx) {
			window.htmx.ajax('POST', action.hxPost, { source: document.body, swap: 'none' });
		}
		this.dismiss();
	},
	dismiss() {
		if (this.done) return;
		this.done = true;
		clearTimeout(this.timeout);
		this.isVisible = false;
		setTimeout(() => {
			this.$dispatch('remove-notification', notification.id);
		}, 300);
	}
}`, ActionMinDuration)
}
