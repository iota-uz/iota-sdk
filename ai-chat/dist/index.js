'use strict';

var React4 = require('react');
var lucideReact = require('lucide-react');
require('clsx');
require('tailwind-merge');

function _interopNamespace(e) {
  if (e && e.__esModule) return e;
  var n = Object.create(null);
  if (e) {
    Object.keys(e).forEach(function (k) {
      if (k !== 'default') {
        var d = Object.getOwnPropertyDescriptor(e, k);
        Object.defineProperty(n, k, d.get ? d : {
          enumerable: true,
          get: function () { return e[k]; }
        });
      }
    });
  }
  n.default = e;
  return Object.freeze(n);
}

var React4__namespace = /*#__PURE__*/_interopNamespace(React4);

var __async = (__this, __arguments, generator) => {
  return new Promise((resolve, reject) => {
    var fulfilled = (value) => {
      try {
        step(generator.next(value));
      } catch (e) {
        reject(e);
      }
    };
    var rejected = (value) => {
      try {
        step(generator.throw(value));
      } catch (e) {
        reject(e);
      }
    };
    var step = (x) => x.done ? resolve(x.value) : Promise.resolve(x.value).then(fulfilled, rejected);
    step((generator = generator.apply(__this, __arguments)).next());
  });
};
function formatDate(date, translations2) {
  const day = date.getDate();
  const month = translations2.months[date.getMonth()];
  const year = date.getFullYear();
  return `${month} ${day}, ${year}`;
}
function CallbackModal({ isOpen, onClose, onSubmit, translations: translations2 }) {
  const [phoneNumber, setPhoneNumber] = React4.useState("");
  const [consentChecked, setConsentChecked] = React4.useState(false);
  if (!isOpen) {
    return null;
  }
  const handleSubmit = () => {
    if (phoneNumber.trim() && consentChecked) {
      onSubmit(phoneNumber);
      onClose();
    }
  };
  return /* @__PURE__ */ React4__namespace.default.createElement("div", { className: "fixed inset-0 bg-black/50 flex items-center justify-center z-50" }, /* @__PURE__ */ React4__namespace.default.createElement("div", { className: "bg-white rounded-3xl w-full max-w-md mx-4 overflow-hidden" }, /* @__PURE__ */ React4__namespace.default.createElement("div", { className: "p-4 flex justify-between items-center" }, /* @__PURE__ */ React4__namespace.default.createElement("span", { className: "text-gray-500 text-sm" }, "Modal"), /* @__PURE__ */ React4__namespace.default.createElement(
    "button",
    {
      onClick: onClose,
      className: "w-8 h-8 flex items-center justify-center rounded-full border border-gray-300"
    },
    /* @__PURE__ */ React4__namespace.default.createElement(lucideReact.X, { size: 16 })
  )), /* @__PURE__ */ React4__namespace.default.createElement("div", { className: "p-6 pt-0" }, /* @__PURE__ */ React4__namespace.default.createElement("h2", { className: "text-2xl font-medium text-[#0a223e] mb-2" }, translations2.callbackModalTitle), /* @__PURE__ */ React4__namespace.default.createElement("p", { className: "text-[#8b98a5] mb-6" }, translations2.callbackModalSubtitle), /* @__PURE__ */ React4__namespace.default.createElement("div", { className: "mb-2 text-[#8b98a5] text-sm" }, translations2.callbackPhoneInputLabel), /* @__PURE__ */ React4__namespace.default.createElement("div", { className: "relative mb-4" }, /* @__PURE__ */ React4__namespace.default.createElement(
    "input",
    {
      type: "text",
      className: "w-full p-3 border border-gray-200 rounded-lg focus:outline-none focus:ring-1 focus:ring-[#2e67b4]",
      placeholder: translations2.phoneInputPlaceholder,
      value: phoneNumber,
      onChange: (e) => setPhoneNumber(e.target.value)
    }
  ), /* @__PURE__ */ React4__namespace.default.createElement("div", { className: "absolute right-3 top-1/2 transform -translate-y-1/2 bg-[#2e67b4] text-white w-6 h-6 rounded-full flex items-center justify-center" }, /* @__PURE__ */ React4__namespace.default.createElement("span", { className: "text-xs" }, "B"))), /* @__PURE__ */ React4__namespace.default.createElement("p", { className: "text-[#2e67b4] text-sm mb-4" }, translations2.dataPrivacyMessage), /* @__PURE__ */ React4__namespace.default.createElement("div", { className: "flex items-start mb-6" }, /* @__PURE__ */ React4__namespace.default.createElement(
    "div",
    {
      className: `w-6 h-6 flex-shrink-0 rounded border ${consentChecked ? "bg-[#2e67b4] border-[#2e67b4] flex items-center justify-center" : "border-gray-300"} mr-2 cursor-pointer`,
      onClick: () => setConsentChecked(!consentChecked)
    },
    consentChecked && /* @__PURE__ */ React4__namespace.default.createElement("span", { className: "text-white text-xs" }, "\u2713")
  ), /* @__PURE__ */ React4__namespace.default.createElement("span", { className: "text-[#0a223e]" }, translations2.dataProcessingConsent)), /* @__PURE__ */ React4__namespace.default.createElement("div", { className: "flex gap-4" }, /* @__PURE__ */ React4__namespace.default.createElement("button", { onClick: onClose, className: "flex-1 py-3 bg-[#e4e9ee] text-[#0a223e] rounded-lg" }, translations2.backButton), /* @__PURE__ */ React4__namespace.default.createElement(
    "button",
    {
      onClick: handleSubmit,
      disabled: !phoneNumber.trim() || !consentChecked,
      className: `flex-1 py-3 rounded-lg ${phoneNumber.trim() && consentChecked ? "bg-[#2e67b4] text-white" : "bg-[#e4e9ee] text-[#bdc8d2]"}`
    },
    translations2.requestCallButton
  )))));
}
function TypingIndicator({ translations: translations2, botTitle }) {
  const [dots, setDots] = React4.useState(".");
  React4.useEffect(() => {
    const interval = setInterval(() => {
      setDots((prev) => {
        if (prev === "...") {
          return ".";
        }
        return prev + ".";
      });
    }, 500);
    return () => clearInterval(interval);
  }, []);
  return /* @__PURE__ */ React4__namespace.default.createElement("div", { className: "max-w-[80%] bg-white rounded-lg p-4 shadow-sm" }, /* @__PURE__ */ React4__namespace.default.createElement("div", { className: "text-[#2e67b4] font-medium mb-2" }, botTitle || translations2.chatbotTitle), /* @__PURE__ */ React4__namespace.default.createElement("div", { className: "flex items-center" }, /* @__PURE__ */ React4__namespace.default.createElement("span", { className: "text-[#8b98a5] font-medium" }, dots)));
}
function QuickReplyButtons({ translations: translations2, isTyping, onQuickReply, faqItems }) {
  const buttonClasses = "flex-1 px-4 py-3 text-[#0a223e] bg-white border border-gray-300 rounded-full text-sm whitespace-normal text-center min-h-[40px] flex items-center justify-center";
  const defaultFAQs = [
    { id: "extend-policy", question: translations2.extendPolicyQuestion },
    { id: "find-contract", question: translations2.findContractNumberQuestion },
    { id: "submit-claim", question: translations2.submitClaimQuestion }
  ];
  const faqs = faqItems || defaultFAQs;
  return /* @__PURE__ */ React4__namespace.default.createElement("div", { className: "flex flex-wrap gap-2 mb-4" }, faqs.length > 0 && faqs.slice(0, 2).map((faq) => /* @__PURE__ */ React4__namespace.default.createElement("button", { key: faq.id, className: buttonClasses, onClick: () => onQuickReply(faq.question), disabled: isTyping }, faq.question)), faqs.length > 2 && /* @__PURE__ */ React4__namespace.default.createElement(
    "button",
    {
      className: "w-full px-4 py-3 text-[#0a223e] bg-white border border-gray-300 rounded-full text-sm whitespace-normal text-center min-h-[40px] flex items-center justify-center mt-2",
      onClick: () => onQuickReply(faqs[2].question),
      disabled: isTyping
    },
    faqs[2].question
  ));
}

// lib/api-service.ts
var ChatApiService = class {
  constructor() {
    this.apiEndpoint = "";
  }
  setApiEndpoint(endpoint) {
    this.apiEndpoint = endpoint;
  }
  createThread(data) {
    return __async(this, null, function* () {
      const response = yield fetch(`${this.apiEndpoint}/messages`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json"
        },
        body: JSON.stringify(data)
      });
      if (!response.ok) {
        const errorData = yield response.json().catch(() => null);
        throw new Error((errorData == null ? void 0 : errorData.error) || (errorData == null ? void 0 : errorData.details) || `API error: ${response.status}`);
      }
      return yield response.json();
    });
  }
  // Get all messages for a thread
  getMessages(threadId) {
    return __async(this, null, function* () {
      const response = yield fetch(`${this.apiEndpoint}/messages/${threadId}`, {
        method: "GET",
        headers: {
          "Content-Type": "application/json"
        }
      });
      if (!response.ok) {
        const errorData = yield response.json().catch(() => null);
        throw new Error((errorData == null ? void 0 : errorData.error) || (errorData == null ? void 0 : errorData.details) || `API error: ${response.status}`);
      }
      return yield response.json();
    });
  }
  // Add a new message to an existing thread
  addMessage(threadId, data) {
    return __async(this, null, function* () {
      const response = yield fetch(`${this.apiEndpoint}/messages/${threadId}`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json"
        },
        body: JSON.stringify(data)
      });
      if (!response.ok) {
        const errorData = yield response.json().catch(() => null);
        throw new Error((errorData == null ? void 0 : errorData.error) || (errorData == null ? void 0 : errorData.details) || `API error: ${response.status}`);
      }
      return yield response.json();
    });
  }
};
var chatApi = new ChatApiService();

// lib/translations.ts
var ru = {
  // Header
  chatbotTitle: "Ai chat bot",
  chatbotSubtitle: "\u041D\u0430\u0448 AI-\u0431\u043E\u0442 \u0433\u043E\u0442\u043E\u0432 \u043F\u043E\u043C\u043E\u0447\u044C \u0432\u0430\u043C \u043A\u0440\u0443\u0433\u043B\u043E\u0441\u0443\u0442\u043E\u0447\u043D\u043E",
  // Welcome message
  welcomeGreeting: "\u041F\u0440\u0438\u0432\u0435\u0442! \u042F \u0432\u0438\u0440\u0442\u0443\u0430\u043B\u044C\u043D\u044B\u0439 \u043F\u043E\u043C\u043E\u0449\u043D\u0438\u043A Euroasia Insurance \u{1F44B}",
  welcomeMessage: "\u0413\u043E\u0442\u043E\u0432 \u043F\u043E\u043C\u043E\u0447\u044C \u0432\u0430\u043C \u0441 \u043E\u0444\u043E\u0440\u043C\u043B\u0435\u043D\u0438\u0435\u043C \u043F\u043E\u043B\u0438\u0441\u0430, \u0440\u0430\u0441\u0447\u0435\u0442\u043E\u043C \u0441\u0442\u043E\u0438\u043C\u043E\u0441\u0442\u0438 \u0438 \u043B\u044E\u0431\u044B\u043C\u0438 \u0432\u043E\u043F\u0440\u043E\u0441\u0430\u043C\u0438.",
  phoneRequestMessage: "\u0427\u0442\u043E\u0431\u044B \u043D\u0430\u0447\u0430\u0442\u044C, \u043F\u043E\u0436\u0430\u043B\u0443\u0439\u0441\u0442\u0430, \u0432\u0432\u0435\u0434\u0438\u0442\u0435 \u0441\u0432\u043E\u0439 \u043D\u043E\u043C\u0435\u0440 \u0442\u0435\u043B\u0435\u0444\u043E\u043D\u0430 \u2014 \u043C\u044B \u0438\u0441\u043F\u043E\u043B\u044C\u0437\u0443\u0435\u043C \u0435\u0433\u043E \u0434\u043B\u044F \u0441\u0432\u044F\u0437\u0438 \u0438 \u0441\u043E\u0445\u0440\u0430\u043D\u0435\u043D\u0438\u044F \u0438\u0441\u0442\u043E\u0440\u0438\u0438 \u043E\u0431\u0440\u0430\u0449\u0435\u043D\u0438\u0439.",
  // Input placeholders
  phoneInputPlaceholder: "+ 998 (__) ___ __ __",
  messageInputPlaceholder: "\u0421\u043E\u043E\u0431\u0449\u0435\u043D\u0438\u044F",
  // Buttons
  sendButton: "\u041E\u0442\u043F\u0440\u0430\u0432\u0438\u0442\u044C",
  callbackRequestButton: "\u0417\u0430\u043F\u0440\u043E\u0441 \u043E\u0431\u0440\u0430\u0442\u043D\u043E\u0433\u043E \u0437\u0432\u043E\u043D\u043A\u0430",
  // Quick replies
  extendPolicyQuestion: "\u041A\u0430\u043A \u043F\u0440\u043E\u0434\u043B\u0438\u0442\u044C \u043F\u043E\u043B\u0438\u0441?",
  findContractNumberQuestion: "\u0413\u0434\u0435 \u043D\u0430\u0439\u0442\u0438 \u043D\u043E\u043C\u0435\u0440 \u0434\u043E\u0433\u043E\u0432\u043E\u0440\u0430?",
  submitClaimQuestion: "\u041A\u0430\u043A \u043F\u043E\u0434\u0430\u0442\u044C \u0437\u0430\u044F\u0432\u043B\u0435\u043D\u0438\u0435 \u043D\u0430 \u0441\u0442\u0440\u0430\u0445\u043E\u0432\u043E\u0439 \u0441\u043B\u0443\u0447\u0430\u0439?",
  // Callback modal
  callbackModalTitle: "\u0417\u0430\u043A\u0430\u0436\u0438\u0442\u0435 \u043E\u0431\u0440\u0430\u0442\u043D\u044B\u0439 \u0437\u0432\u043E\u043D\u043E\u043A",
  callbackModalSubtitle: "\u041E\u0441\u0442\u0430\u0432\u044C\u0442\u0435 \u0441\u0432\u043E\u0439 \u043D\u043E\u043C\u0435\u0440 \u0442\u0435\u043B\u0435\u0444\u043E\u043D\u0430, \u0438 \u043D\u0430\u0448 \u0441\u043F\u0435\u0446\u0438\u0430\u043B\u0438\u0441\u0442 \u0441\u0432\u044F\u0436\u0435\u0442\u0441\u044F \u0441 \u0432\u0430\u043C\u0438 \u0432 \u0431\u043B\u0438\u0436\u0430\u0439\u0448\u0435\u0435 \u0432\u0440\u0435\u043C\u044F",
  callbackPhoneInputLabel: "\u0412\u0432\u0435\u0434\u0438\u0442\u0435 \u043D\u043E\u043C\u0435\u0440 \u0442\u0435\u043B\u0435\u0444\u043E\u043D\u0430",
  dataPrivacyMessage: "\u041C\u044B \u043D\u0435 \u043F\u0435\u0440\u0435\u0434\u0430\u0451\u043C \u0432\u0430\u0448\u0438 \u0434\u0430\u043D\u043D\u044B\u0435 \u0442\u0440\u0435\u0442\u044C\u0438\u043C \u043B\u0438\u0446\u0430\u043C",
  dataProcessingConsent: "\u0421\u043E\u0433\u043B\u0430\u0441\u0435\u043D(\u0430) \u0441 \u043E\u0431\u0440\u0430\u0431\u043E\u0442\u043A\u043E\u0439 \u043F\u0435\u0440\u0441\u043E\u043D\u0430\u043B\u044C\u043D\u044B\u0445 \u0434\u0430\u043D\u043D\u044B\u0445",
  backButton: "\u041D\u0430\u0437\u0430\u0434",
  requestCallButton: "\u0417\u0430\u043A\u0430\u0437\u0430\u0442\u044C \u0437\u0432\u043E\u043D\u043E\u043A",
  // Messages
  callbackConfirmation: "\u0421\u043F\u0430\u0441\u0438\u0431\u043E \u0437\u0430 \u0437\u0430\u043F\u0440\u043E\u0441! \u041D\u0430\u0448 \u0441\u043F\u0435\u0446\u0438\u0430\u043B\u0438\u0441\u0442 \u0441\u0432\u044F\u0436\u0435\u0442\u0441\u044F \u0441 \u0432\u0430\u043C\u0438 \u043F\u043E \u043D\u043E\u043C\u0435\u0440\u0443 {phone} \u0432 \u0431\u043B\u0438\u0436\u0430\u0439\u0448\u0435\u0435 \u0432\u0440\u0435\u043C\u044F.",
  errorLoadingMessages: "\u041D\u0435 \u0443\u0434\u0430\u043B\u043E\u0441\u044C \u0437\u0430\u0433\u0440\u0443\u0437\u0438\u0442\u044C \u0438\u0441\u0442\u043E\u0440\u0438\u044E \u0441\u043E\u043E\u0431\u0449\u0435\u043D\u0438\u0439. \u041F\u043E\u0436\u0430\u043B\u0443\u0439\u0441\u0442\u0430, \u043F\u043E\u043F\u0440\u043E\u0431\u0443\u0439\u0442\u0435 \u0435\u0449\u0435 \u0440\u0430\u0437 \u043F\u043E\u0437\u0436\u0435.",
  errorCreatingChat: "\u041F\u0440\u043E\u0438\u0437\u043E\u0448\u043B\u0430 \u043E\u0448\u0438\u0431\u043A\u0430 \u043F\u0440\u0438 \u0441\u043E\u0437\u0434\u0430\u043D\u0438\u0438 \u0447\u0430\u0442\u0430. \u041F\u043E\u0436\u0430\u043B\u0443\u0439\u0441\u0442\u0430, \u043F\u043E\u043F\u0440\u043E\u0431\u0443\u0439\u0442\u0435 \u0435\u0449\u0435 \u0440\u0430\u0437 \u043F\u043E\u0437\u0436\u0435.",
  errorSendingMessage: "\u0418\u0437\u0432\u0438\u043D\u0438\u0442\u0435, \u043F\u0440\u043E\u0438\u0437\u043E\u0448\u043B\u0430 \u043E\u0448\u0438\u0431\u043A\u0430 \u043F\u0440\u0438 \u043E\u0442\u043F\u0440\u0430\u0432\u043A\u0435 \u0441\u043E\u043E\u0431\u0449\u0435\u043D\u0438\u044F. \u041F\u043E\u0436\u0430\u043B\u0443\u0439\u0441\u0442\u0430, \u043F\u043E\u043F\u0440\u043E\u0431\u0443\u0439\u0442\u0435 \u0435\u0449\u0435 \u0440\u0430\u0437 \u043F\u043E\u0437\u0436\u0435.",
  threadNotFoundMessage: "\u0412\u0430\u0448 \u0447\u0430\u0442 \u0431\u044B\u043B \u0437\u0430\u0432\u0435\u0440\u0448\u0435\u043D \u0438\u043B\u0438 \u043D\u0435 \u043D\u0430\u0439\u0434\u0435\u043D. \u041D\u0430\u0447\u0438\u043D\u0430\u0435\u043C \u043D\u043E\u0432\u044B\u0439 \u0447\u0430\u0442.",
  // Date formatting
  months: [
    "\u042F\u043D\u0432\u0430\u0440\u044C",
    "\u0424\u0435\u0432\u0440\u0430\u043B\u044C",
    "\u041C\u0430\u0440\u0442",
    "\u0410\u043F\u0440\u0435\u043B\u044C",
    "\u041C\u0430\u0439",
    "\u0418\u044E\u043D\u044C",
    "\u0418\u044E\u043B\u044C",
    "\u0410\u0432\u0433\u0443\u0441\u0442",
    "\u0421\u0435\u043D\u0442\u044F\u0431\u0440\u044C",
    "\u041E\u043A\u0442\u044F\u0431\u0440\u044C",
    "\u041D\u043E\u044F\u0431\u0440\u044C",
    "\u0414\u0435\u043A\u0430\u0431\u0440\u044C"
  ]
};
var uz = {
  // Header
  chatbotTitle: "AI suhbat boti",
  chatbotSubtitle: "Bizning AI-botimiz sizga 24/7 yordam berishga tayyor",
  // Welcome message
  welcomeGreeting: "Salom! Men Euroasia Insurance virtual yordamchisiman \u{1F44B}",
  welcomeMessage: "Sug'urta polisini rasmiylashtirish, narxni hisoblash va har qanday savollar bo'yicha yordam berishga tayyorman.",
  phoneRequestMessage: "Boshlash uchun, iltimos, telefon raqamingizni kiriting \u2014 biz undan aloqa va murojaat tarixini saqlash uchun foydalanamiz.",
  // Input placeholders
  phoneInputPlaceholder: "+ 998 (__) ___ __ __",
  messageInputPlaceholder: "Xabarlar",
  // Buttons
  sendButton: "Yuborish",
  callbackRequestButton: "Qayta qo'ng'iroq so'rovi",
  // Quick replies
  extendPolicyQuestion: "Polisni qanday uzaytirish mumkin?",
  findContractNumberQuestion: "Shartnoma raqamini qayerdan topish mumkin?",
  submitClaimQuestion: "Sug'urta hodisasi bo'yicha arizani qanday topshirish kerak?",
  // Callback modal
  callbackModalTitle: "Qayta qo'ng'iroq buyurtma qiling",
  callbackModalSubtitle: "Telefon raqamingizni qoldiring, va bizning mutaxassisimiz siz bilan yaqin vaqt ichida bog'lanadi",
  callbackPhoneInputLabel: "Telefon raqamini kiriting",
  dataPrivacyMessage: "Biz sizning ma'lumotlaringizni uchinchi shaxslarga bermaydi",
  dataProcessingConsent: "Shaxsiy ma'lumotlarni qayta ishlashga roziman",
  backButton: "Orqaga",
  requestCallButton: "Qo'ng'iroq buyurtma qilish",
  // Messages
  callbackConfirmation: "So'rov uchun rahmat! Mutaxassisimiz {phone} raqami orqali siz bilan yaqin vaqt ichida bog'lanadi.",
  errorLoadingMessages: "Xabarlar tarixini yuklab bo'lmadi. Iltimos, keyinroq qayta urinib ko'ring.",
  errorCreatingChat: "Chat yaratishda xatolik yuz berdi. Iltimos, keyinroq qayta urinib ko'ring.",
  errorSendingMessage: "Kechirasiz, xabar yuborishda xatolik yuz berdi. Iltimos, keyinroq qayta urinib ko'ring.",
  threadNotFoundMessage: "Sizning chatingiz tugatilgan yoki topilmadi. Yangi chat boshlaymiz.",
  // Date formatting
  months: [
    "Yanvar",
    "Fevral",
    "Mart",
    "Aprel",
    "May",
    "Iyun",
    "Iyul",
    "Avgust",
    "Sentabr",
    "Oktabr",
    "Noyabr",
    "Dekabr"
  ]
};
var uzCyrl = {
  // Header
  chatbotTitle: "AI \u0441\u0443\u04B3\u0431\u0430\u0442 \u0431\u043E\u0442\u0438",
  chatbotSubtitle: "\u0411\u0438\u0437\u043D\u0438\u043D\u0433 AI-\u0431\u043E\u0442\u0438\u043C\u0438\u0437 \u0441\u0438\u0437\u0433\u0430 24/7 \u0451\u0440\u0434\u0430\u043C \u0431\u0435\u0440\u0438\u0448\u0433\u0430 \u0442\u0430\u0439\u0451\u0440",
  // Welcome message
  welcomeGreeting: "\u0421\u0430\u043B\u043E\u043C! \u041C\u0435\u043D Euroasia Insurance \u0432\u0438\u0440\u0442\u0443\u0430\u043B \u0451\u0440\u0434\u0430\u043C\u0447\u0438\u0441\u0438\u043C\u0430\u043D \u{1F44B}",
  welcomeMessage: "\u0421\u0443\u0493\u0443\u0440\u0442\u0430 \u043F\u043E\u043B\u0438\u0441\u0438\u043D\u0438 \u0440\u0430\u0441\u043C\u0438\u0439\u043B\u0430\u0448\u0442\u0438\u0440\u0438\u0448, \u043D\u0430\u0440\u0445\u043D\u0438 \u04B3\u0438\u0441\u043E\u0431\u043B\u0430\u0448 \u0432\u0430 \u04B3\u0430\u0440 \u049B\u0430\u043D\u0434\u0430\u0439 \u0441\u0430\u0432\u043E\u043B\u043B\u0430\u0440 \u0431\u045E\u0439\u0438\u0447\u0430 \u0451\u0440\u0434\u0430\u043C \u0431\u0435\u0440\u0438\u0448\u0433\u0430 \u0442\u0430\u0439\u0451\u0440\u043C\u0430\u043D.",
  phoneRequestMessage: "\u0411\u043E\u0448\u043B\u0430\u0448 \u0443\u0447\u0443\u043D, \u0438\u043B\u0442\u0438\u043C\u043E\u0441, \u0442\u0435\u043B\u0435\u0444\u043E\u043D \u0440\u0430\u049B\u0430\u043C\u0438\u043D\u0433\u0438\u0437\u043D\u0438 \u043A\u0438\u0440\u0438\u0442\u0438\u043D\u0433 \u2014 \u0431\u0438\u0437 \u0443\u043D\u0434\u0430\u043D \u0430\u043B\u043E\u049B\u0430 \u0432\u0430 \u043C\u0443\u0440\u043E\u0436\u0430\u0430\u0442 \u0442\u0430\u0440\u0438\u0445\u0438\u043D\u0438 \u0441\u0430\u049B\u043B\u0430\u0448 \u0443\u0447\u0443\u043D \u0444\u043E\u0439\u0434\u0430\u043B\u0430\u043D\u0430\u043C\u0438\u0437.",
  // Input placeholders
  phoneInputPlaceholder: "+ 998 (__) ___ __ __",
  messageInputPlaceholder: "\u0425\u0430\u0431\u0430\u0440\u043B\u0430\u0440",
  // Buttons
  sendButton: "\u042E\u0431\u043E\u0440\u0438\u0448",
  callbackRequestButton: "\u049A\u0430\u0439\u0442\u0430 \u049B\u045E\u043D\u0493\u0438\u0440\u043E\u049B \u0441\u045E\u0440\u043E\u0432\u0438",
  // Quick replies
  extendPolicyQuestion: "\u041F\u043E\u043B\u0438\u0441\u043D\u0438 \u049B\u0430\u043D\u0434\u0430\u0439 \u0443\u0437\u0430\u0439\u0442\u0438\u0440\u0438\u0448 \u043C\u0443\u043C\u043A\u0438\u043D?",
  findContractNumberQuestion: "\u0428\u0430\u0440\u0442\u043D\u043E\u043C\u0430 \u0440\u0430\u049B\u0430\u043C\u0438\u043D\u0438 \u049B\u0430\u0435\u0440\u0434\u0430\u043D \u0442\u043E\u043F\u0438\u0448 \u043C\u0443\u043C\u043A\u0438\u043D?",
  submitClaimQuestion: "\u0421\u0443\u0493\u0443\u0440\u0442\u0430 \u04B3\u043E\u0434\u0438\u0441\u0430\u0441\u0438 \u0431\u045E\u0439\u0438\u0447\u0430 \u0430\u0440\u0438\u0437\u0430\u043D\u0438 \u049B\u0430\u043D\u0434\u0430\u0439 \u0442\u043E\u043F\u0448\u0438\u0440\u0438\u0448 \u043A\u0435\u0440\u0430\u043A?",
  // Callback modal
  callbackModalTitle: "\u049A\u0430\u0439\u0442\u0430 \u049B\u045E\u043D\u0493\u0438\u0440\u043E\u049B \u0431\u0443\u044E\u0440\u0442\u043C\u0430 \u049B\u0438\u043B\u0438\u043D\u0433",
  callbackModalSubtitle: "\u0422\u0435\u043B\u0435\u0444\u043E\u043D \u0440\u0430\u049B\u0430\u043C\u0438\u043D\u0433\u0438\u0437\u043D\u0438 \u049B\u043E\u043B\u0434\u0438\u0440\u0438\u043D\u0433, \u0432\u0430 \u0431\u0438\u0437\u043D\u0438\u043D\u0433 \u043C\u0443\u0442\u0430\u0445\u0430\u0441\u0441\u0438\u0441\u0438\u043C\u0438\u0437 \u0441\u0438\u0437 \u0431\u0438\u043B\u0430\u043D \u044F\u049B\u0438\u043D \u0432\u0430\u049B\u0442 \u0438\u0447\u0438\u0434\u0430 \u0431\u043E\u0493\u043B\u0430\u043D\u0430\u0434\u0438",
  callbackPhoneInputLabel: "\u0422\u0435\u043B\u0435\u0444\u043E\u043D \u0440\u0430\u049B\u0430\u043C\u0438\u043D\u0438 \u043A\u0438\u0440\u0438\u0442\u0438\u043D\u0433",
  dataPrivacyMessage: "\u0411\u0438\u0437 \u0441\u0438\u0437\u043D\u0438\u043D\u0433 \u043C\u0430\u044A\u043B\u0443\u043C\u043E\u0442\u043B\u0430\u0440\u0438\u043D\u0433\u0438\u0437\u043D\u0438 \u0443\u0447\u0438\u043D\u0447\u0438 \u0448\u0430\u0445\u0441\u043B\u0430\u0440\u0433\u0430 \u0431\u0435\u0440\u043C\u0430\u0439\u0434\u0438",
  dataProcessingConsent: "\u0428\u0430\u0445\u0441\u0438\u0439 \u043C\u0430\u044A\u043B\u0443\u043C\u043E\u0442\u043B\u0430\u0440\u043D\u0438 \u049B\u0430\u0439\u0442\u0430 \u0438\u0448\u043B\u0430\u0448\u0433\u0430 \u0440\u043E\u0437\u0438\u043C\u0430\u043D",
  backButton: "\u041E\u0440\u049B\u0430\u0433\u0430",
  requestCallButton: "\u049A\u045E\u043D\u0493\u0438\u0440\u043E\u049B \u0431\u0443\u044E\u0440\u0442\u043C\u0430 \u049B\u0438\u043B\u0438\u0448",
  // Messages
  callbackConfirmation: "\u0421\u045E\u0440\u043E\u0432 \u0443\u0447\u0443\u043D \u0440\u0430\u04B3\u043C\u0430\u0442! \u041C\u0443\u0442\u0430\u0445\u0430\u0441\u0441\u0438\u0441\u0438\u043C\u0438\u0437 {phone} \u0440\u0430\u049B\u0430\u043C\u0438 \u043E\u0440\u049B\u0430\u043B\u0438 \u0441\u0438\u0437 \u0431\u0438\u043B\u0430\u043D \u044F\u049B\u0438\u043D \u0432\u0430\u049B\u0442 \u0438\u0447\u0438\u0434\u0430 \u0431\u043E\u0493\u043B\u0430\u043D\u0430\u0434\u0438.",
  errorLoadingMessages: "\u0425\u0430\u0431\u0430\u0440\u043B\u0430\u0440 \u0442\u0430\u0440\u0438\u0445\u0438\u043D\u0438 \u044E\u043A\u043B\u0430\u0431 \u0431\u045E\u043B\u043C\u0430\u0434\u0438. \u0418\u043B\u0442\u0438\u043C\u043E\u0441, \u043A\u0435\u0439\u0438\u043D\u0440\u043E\u049B \u049B\u0430\u0439\u0442\u0430 \u0443\u0440\u0438\u043D\u0438\u0431 \u043A\u045E\u0440\u0438\u043D\u0433.",
  errorCreatingChat: "\u0427\u0430\u0442 \u044F\u0440\u0430\u0442\u0438\u0448\u0434\u0430 \u0445\u0430\u0442\u043E\u043B\u0438\u043A \u044E\u0437 \u0431\u0435\u0440\u0434\u0438. \u0418\u043B\u0442\u0438\u043C\u043E\u0441, \u043A\u0435\u0439\u0438\u043D\u0440\u043E\u049B \u049B\u0430\u0439\u0442\u0430 \u0443\u0440\u0438\u043D\u0438\u0431 \u043A\u045E\u0440\u0438\u043D\u0433.",
  errorSendingMessage: "\u041A\u0435\u0447\u0438\u0440\u0430\u0441\u0438\u0437, \u0445\u0430\u0431\u0430\u0440 \u044E\u0431\u043E\u0440\u0438\u0448\u0434\u0430 \u0445\u0430\u0442\u043E\u043B\u0438\u043A \u044E\u0437 \u0431\u0435\u0440\u0434\u0438. \u0418\u043B\u0442\u0438\u043C\u043E\u0441, \u043A\u0435\u0439\u0438\u043D\u0440\u043E\u049B \u049B\u0430\u0439\u0442\u0430 \u0443\u0440\u0438\u043D\u0438\u0431 \u043A\u045E\u0440\u0438\u043D\u0433.",
  threadNotFoundMessage: "\u0421\u0438\u0437\u043D\u0438\u043D\u0433 \u0447\u0430\u0442\u0438\u043D\u0433\u0438\u0437 \u0442\u0443\u0433\u0430\u0442\u0438\u043B\u0433\u0430\u043D \u0451\u043A\u0438 \u0442\u043E\u043F\u0438\u043B\u043C\u0430\u0434\u0438. \u042F\u043D\u0433\u0438 \u0447\u0430\u0442 \u0431\u043E\u0448\u043B\u0430\u0439\u043C\u0438\u0437.",
  // Date formatting
  months: [
    "\u042F\u043D\u0432\u0430\u0440\u044C",
    "\u0424\u0435\u0432\u0440\u0430\u043B\u044C",
    "\u041C\u0430\u0440\u0442",
    "\u0410\u043F\u0440\u0435\u043B\u044C",
    "\u041C\u0430\u0439",
    "\u0418\u044E\u043D\u044C",
    "\u0418\u044E\u043B\u044C",
    "\u0410\u0432\u0433\u0443\u0441\u0442",
    "\u0421\u0435\u043D\u0442\u044F\u0431\u0440\u044C",
    "\u041E\u043A\u0442\u044F\u0431\u0440\u044C",
    "\u041D\u043E\u044F\u0431\u0440\u044C",
    "\u0414\u0435\u043A\u0430\u0431\u0440\u044C"
  ]
};
var en = {
  // Header
  chatbotTitle: "AI chat bot",
  chatbotSubtitle: "Our AI bot is ready to help you 24/7",
  // Welcome message
  welcomeGreeting: "Hello! I'm the virtual assistant of Euroasia Insurance \u{1F44B}",
  welcomeMessage: "I'm ready to help you with policy registration, cost calculation, and any questions you may have.",
  phoneRequestMessage: "To get started, please enter your phone number \u2014 we use it for communication and to save your request history.",
  // Input placeholders
  phoneInputPlaceholder: "+ 998 (__) ___ __ __",
  messageInputPlaceholder: "Messages",
  // Buttons
  sendButton: "Send",
  callbackRequestButton: "Request a callback",
  // Quick replies
  extendPolicyQuestion: "How to extend my policy?",
  findContractNumberQuestion: "Where to find my contract number?",
  submitClaimQuestion: "How to submit an insurance claim?",
  // Callback modal
  callbackModalTitle: "Request a callback",
  callbackModalSubtitle: "Leave your phone number, and our specialist will contact you shortly",
  callbackPhoneInputLabel: "Enter phone number",
  dataPrivacyMessage: "We don't share your data with third parties",
  dataProcessingConsent: "I agree to the processing of personal data",
  backButton: "Back",
  requestCallButton: "Request call",
  // Messages
  callbackConfirmation: "Thank you for your request! Our specialist will contact you at {phone} shortly.",
  errorLoadingMessages: "Failed to load message history. Please try again later.",
  errorCreatingChat: "An error occurred while creating the chat. Please try again later.",
  errorSendingMessage: "Sorry, an error occurred while sending the message. Please try again later.",
  threadNotFoundMessage: "Your chat has been ended or not found. Starting a new chat.",
  // Date formatting
  months: [
    "January",
    "February",
    "March",
    "April",
    "May",
    "June",
    "July",
    "August",
    "September",
    "October",
    "November",
    "December"
  ]
};
var translations = {
  ru,
  uz,
  uzCyrl,
  en
};
function getTranslations(locale) {
  return translations[locale] || en;
}

// components/chatbot-interface.tsx
var MessageBubble = ({ message, translations: translations2, botTitle }) => {
  return /* @__PURE__ */ React4__namespace.createElement(
    "div",
    {
      className: `max-w-[80%] w-fit ${message.sender === "user" ? "ml-auto bg-[#dce6f3] rounded-tl-2xl rounded-tr-2xl rounded-bl-2xl p-3" : "bg-white rounded-tr-2xl rounded-tl-2xl rounded-br-xl p-4 shadow-sm"}`
    },
    message.sender === "bot" && /* @__PURE__ */ React4__namespace.createElement("div", { className: "text-[#2e67b4] font-medium mb-2" }, botTitle || translations2.chatbotTitle),
    /* @__PURE__ */ React4__namespace.createElement("p", { className: "whitespace-pre-line" }, message.content)
  );
};
function ChatbotInterface({
  locale = "ru",
  apiEndpoint,
  // Direct API endpoint (required)
  faqItems,
  title,
  subtitle
}) {
  const translations2 = getTranslations(locale);
  React4.useEffect(() => {
    if (apiEndpoint) {
      chatApi.setApiEndpoint(apiEndpoint);
    }
  }, [apiEndpoint]);
  const [phoneSubmitted, setPhoneSubmitted] = React4.useState(false);
  const [phoneNumber, setPhoneNumber] = React4.useState("");
  const [messages, setMessages] = React4.useState([]);
  const [currentMessage, setCurrentMessage] = React4.useState("");
  const [showDateHeader, setShowDateHeader] = React4.useState(false);
  const [isCallbackModalOpen, setIsCallbackModalOpen] = React4.useState(false);
  const [isTyping, setIsTyping] = React4.useState(false);
  const [threadId, setThreadId] = React4.useState(null);
  const [error, setError] = React4.useState(null);
  const messagesEndRef = React4.useRef(null);
  const [isOpen, setIsOpen] = React4.useState(true);
  const [windowHeight, setWindowHeight] = React4.useState(0);
  const chatbotTitle = title || translations2.chatbotTitle;
  const chatbotSubtitle = subtitle || translations2.chatbotSubtitle;
  const handleResetChat = React4__namespace.useCallback(() => {
    localStorage.removeItem("chatThreadId");
    localStorage.removeItem("chatPhoneNumber");
    setThreadId(null);
    setPhoneSubmitted(false);
    setPhoneNumber("");
    setError(null);
    setMessages([
      {
        id: "welcome",
        content: `${translations2.welcomeGreeting}

${translations2.welcomeMessage}`,
        sender: "bot",
        timestamp: /* @__PURE__ */ new Date()
      }
    ]);
  }, [translations2, setThreadId, setPhoneSubmitted, setPhoneNumber, setError, setMessages]);
  const handle404Error = React4__namespace.useCallback(() => {
    const errorMsg = {
      id: `bot-error-${Date.now()}`,
      content: translations2.threadNotFoundMessage,
      sender: "bot",
      timestamp: /* @__PURE__ */ new Date()
    };
    setMessages([errorMsg]);
    handleResetChat();
  }, [translations2, setMessages, handleResetChat]);
  const fetchMessages = React4__namespace.useCallback((threadId2) => __async(null, null, function* () {
    try {
      setIsTyping(true);
      setError(null);
      const response = yield chatApi.getMessages(threadId2);
      const chatMessages = response.messages.map((msg, index) => ({
        id: `${msg.role}-${index}`,
        content: msg.message,
        sender: msg.role === "user" ? "user" : "bot",
        timestamp: /* @__PURE__ */ new Date()
        // API doesn't provide timestamps, so we use current time
      }));
      setMessages(chatMessages);
    } catch (error2) {
      if (error2 instanceof Error && error2.message.includes("404")) {
        handle404Error();
        return;
      }
      const errorMessage = error2 instanceof Error ? error2.message : "Unknown error";
      setError(`${translations2.errorLoadingMessages}: ${errorMessage}`);
      setMessages([
        {
          id: "error",
          content: translations2.errorLoadingMessages,
          sender: "bot",
          timestamp: /* @__PURE__ */ new Date()
        }
      ]);
    } finally {
      setIsTyping(false);
    }
  }), [setIsTyping, setError, setMessages, handle404Error, translations2]);
  React4.useEffect(() => {
    const updateWindowHeight = () => {
      setWindowHeight(window.innerHeight);
    };
    updateWindowHeight();
    window.addEventListener("resize", updateWindowHeight);
    return () => window.removeEventListener("resize", updateWindowHeight);
  }, []);
  React4.useEffect(() => {
    const storedThreadId = localStorage.getItem("chatThreadId");
    const storedPhone = localStorage.getItem("chatPhoneNumber");
    if (storedThreadId && storedPhone) {
      setThreadId(storedThreadId);
      setPhoneNumber(storedPhone);
      setPhoneSubmitted(true);
      setShowDateHeader(true);
      fetchMessages(storedThreadId);
    } else {
      setMessages([
        {
          id: "welcome",
          content: `${translations2.welcomeGreeting}

${translations2.welcomeMessage}`,
          sender: "bot",
          timestamp: /* @__PURE__ */ new Date()
        }
      ]);
    }
    if (storedThreadId) {
      localStorage.removeItem("newThreadId");
    }
  }, [translations2, fetchMessages]);
  React4.useEffect(() => {
    var _a;
    (_a = messagesEndRef.current) == null ? void 0 : _a.scrollIntoView({ behavior: "smooth" });
  }, [messages, isTyping]);
  const handlePhoneSubmit = () => __async(null, null, function* () {
    if (phoneNumber.trim().length === 0) {
      return;
    }
    try {
      setIsTyping(true);
      setError(null);
      const response = yield chatApi.createThread({
        message: "",
        // Empty message instead of hardcoded text
        phone: phoneNumber
      });
      setThreadId(response.thread_id);
      localStorage.setItem("newThreadId", response.thread_id);
      localStorage.setItem("chatThreadId", response.thread_id);
      localStorage.setItem("chatPhoneNumber", phoneNumber);
      setPhoneSubmitted(true);
      setShowDateHeader(true);
      setMessages([]);
      yield fetchMessages(response.thread_id);
    } catch (error2) {
      const errorMessage = error2 instanceof Error ? error2.message : "Unknown error";
      setError(`${translations2.errorCreatingChat}: ${errorMessage}`);
      setMessages([
        {
          id: "error",
          content: translations2.errorCreatingChat,
          sender: "bot",
          timestamp: /* @__PURE__ */ new Date()
        }
      ]);
    } finally {
      setIsTyping(false);
    }
  });
  const handleSendMessage = () => __async(null, null, function* () {
    if (currentMessage.trim().length === 0 || !threadId) {
      return;
    }
    const userMessage = {
      id: `user-${Date.now()}`,
      content: currentMessage,
      sender: "user",
      timestamp: /* @__PURE__ */ new Date()
    };
    setMessages((prev) => [...prev, userMessage]);
    const messageToSend = currentMessage;
    setCurrentMessage("");
    setError(null);
    setIsTyping(true);
    try {
      yield chatApi.addMessage(threadId, {
        message: messageToSend
      });
      const response = yield chatApi.getMessages(threadId);
      const assistantMessages = response.messages.filter((msg) => msg.role === "assistant");
      if (assistantMessages.length > 0) {
        const latestAssistantMessage = assistantMessages[assistantMessages.length - 1];
        const botMessage = {
          id: `bot-${Date.now()}`,
          content: latestAssistantMessage.message,
          sender: "bot",
          timestamp: /* @__PURE__ */ new Date()
        };
        setMessages((prev) => {
          const isDuplicate = prev.some((msg) => msg.sender === "bot" && msg.content === latestAssistantMessage.message);
          if (isDuplicate) {
            return prev;
          }
          return [...prev, botMessage];
        });
      }
    } catch (error2) {
      if (error2 instanceof Error && error2.message.includes("404")) {
        handle404Error();
        return;
      }
      const errorMessage = error2 instanceof Error ? error2.message : "Unknown error";
      setError(`${translations2.errorSendingMessage}: ${errorMessage}`);
      const errorMsg = {
        id: `bot-error-${Date.now()}`,
        content: translations2.errorSendingMessage,
        sender: "bot",
        timestamp: /* @__PURE__ */ new Date()
      };
      setMessages((prev) => [...prev, errorMsg]);
    } finally {
      setIsTyping(false);
    }
  });
  const handleQuickReply = (question) => {
    setCurrentMessage(question);
    setTimeout(() => {
      handleSendMessage();
    }, 100);
  };
  const handleKeyPress = (e) => {
    if (e.key === "Enter") {
      phoneSubmitted ? handleSendMessage() : handlePhoneSubmit();
    }
  };
  const handleCallbackSubmit = (callbackPhone) => {
    const botMessage = {
      id: `bot-${Date.now()}`,
      content: translations2.callbackConfirmation.replace("{phone}", callbackPhone),
      sender: "bot",
      timestamp: /* @__PURE__ */ new Date()
    };
    setMessages((prev) => [...prev, botMessage]);
  };
  const chatWidth = 450;
  const headerHeight = 60;
  const maxChatHeight = windowHeight ? Math.min(windowHeight * 0.8, 700) : 600;
  const contentHeight = maxChatHeight - headerHeight;
  return /* @__PURE__ */ React4__namespace.createElement("div", { className: "fixed bottom-4 right-4 z-50" }, /* @__PURE__ */ React4__namespace.createElement(
    "div",
    {
      className: `overflow-hidden bg-white rounded-lg shadow-lg transition-all duration-300 ${isOpen ? "opacity-100" : "opacity-95"}`,
      style: {
        width: `${chatWidth}px`,
        height: isOpen ? `${maxChatHeight}px` : `${headerHeight}px`
      }
    },
    /* @__PURE__ */ React4__namespace.createElement(
      "div",
      {
        className: "relative bg-[#0a223e] text-white p-4 flex items-center cursor-pointer",
        style: { height: `${headerHeight}px` },
        onClick: () => setIsOpen(!isOpen)
      },
      /* @__PURE__ */ React4__namespace.createElement("div", { className: "w-10 h-10 bg-[#8b98a5] rounded-full flex items-center justify-center mr-3" }, /* @__PURE__ */ React4__namespace.createElement("span", { className: "text-white" }, "\u2022\u2022\u2022")),
      /* @__PURE__ */ React4__namespace.createElement("div", null, /* @__PURE__ */ React4__namespace.createElement("h1", { className: "text-xl font-medium" }, chatbotTitle), /* @__PURE__ */ React4__namespace.createElement("p", { className: "text-sm opacity-90" }, chatbotSubtitle)),
      /* @__PURE__ */ React4__namespace.createElement(
        lucideReact.ChevronDown,
        {
          className: `absolute right-4 top-1/2 transform -translate-y-1/2 transition-transform duration-300 ${isOpen ? "" : "rotate-180"}`
        }
      )
    ),
    isOpen && /* @__PURE__ */ React4__namespace.createElement("div", { className: "flex flex-col", style: { height: `${contentHeight}px` } }, /* @__PURE__ */ React4__namespace.createElement("div", { className: "bg-[#f2f5f8] p-4 flex-grow overflow-y-auto" }, !phoneSubmitted ? /* @__PURE__ */ React4__namespace.createElement("div", { className: "bg-white rounded-tr-2xl rounded-tl-2xl rounded-br-xl p-4 shadow-sm" }, /* @__PURE__ */ React4__namespace.createElement("div", { className: "text-[#2e67b4] font-medium mb-2" }, chatbotTitle), /* @__PURE__ */ React4__namespace.createElement("p", { className: "mb-2" }, translations2.welcomeGreeting), /* @__PURE__ */ React4__namespace.createElement("p", { className: "mb-4" }, translations2.welcomeMessage), /* @__PURE__ */ React4__namespace.createElement("p", { className: "flex items-start" }, /* @__PURE__ */ React4__namespace.createElement("span", { className: "inline-block mr-2 mt-1" }, "\u{1F512}"), /* @__PURE__ */ React4__namespace.createElement("span", null, translations2.phoneRequestMessage))) : /* @__PURE__ */ React4__namespace.createElement("div", { className: "space-y-4" }, showDateHeader && /* @__PURE__ */ React4__namespace.createElement("div", { className: "text-center text-[#8b98a5] text-sm py-2" }, formatDate(/* @__PURE__ */ new Date(), translations2)), error && /* @__PURE__ */ React4__namespace.createElement("div", { className: "bg-red-100 border border-red-400 text-red-700 px-4 py-3 rounded relative" }, /* @__PURE__ */ React4__namespace.createElement("span", { className: "block sm:inline" }, error)), messages.map((message) => /* @__PURE__ */ React4__namespace.createElement(
      MessageBubble,
      {
        key: message.id,
        message,
        translations: translations2,
        botTitle: chatbotTitle
      }
    )), isTyping && /* @__PURE__ */ React4__namespace.createElement(TypingIndicator, { translations: translations2, botTitle: chatbotTitle }), /* @__PURE__ */ React4__namespace.createElement("div", { ref: messagesEndRef }))), /* @__PURE__ */ React4__namespace.createElement("div", { className: "p-4 bg-white shrink-0" }, !phoneSubmitted ? (
      /* Phone Input */
      /* @__PURE__ */ React4__namespace.createElement("div", { className: "flex items-center p-3 mb-4 bg-[#f2f5f8] rounded-lg" }, /* @__PURE__ */ React4__namespace.createElement(
        "input",
        {
          type: "text",
          className: "bg-transparent focus:outline-none text-[#0a223e] flex-1",
          placeholder: translations2.phoneInputPlaceholder,
          value: phoneNumber,
          onChange: (e) => setPhoneNumber(e.target.value),
          onKeyDown: handleKeyPress
        }
      ), /* @__PURE__ */ React4__namespace.createElement("button", { onClick: handlePhoneSubmit, disabled: isTyping }, /* @__PURE__ */ React4__namespace.createElement(lucideReact.Send, { className: `ml-auto ${isTyping ? "text-[#8b98a5]" : "text-[#0a223e]"}`, size: 20 })))
    ) : (
      /* Message Input */
      /* @__PURE__ */ React4__namespace.createElement("div", { className: "flex items-center p-3 mb-4 bg-[#f2f5f8] rounded-lg" }, /* @__PURE__ */ React4__namespace.createElement(
        "input",
        {
          type: "text",
          className: "bg-transparent focus:outline-none text-[#0a223e] flex-1",
          placeholder: translations2.messageInputPlaceholder,
          value: currentMessage,
          onChange: (e) => setCurrentMessage(e.target.value),
          onKeyDown: handleKeyPress,
          disabled: isTyping
        }
      ), /* @__PURE__ */ React4__namespace.createElement("button", { onClick: handleSendMessage, disabled: isTyping || currentMessage.trim().length === 0 }, /* @__PURE__ */ React4__namespace.createElement(
        lucideReact.Send,
        {
          className: `ml-auto ${currentMessage.trim() && !isTyping ? "text-[#0a223e]" : "text-[#8b98a5]"}`,
          size: 20
        }
      )))
    ), (!threadId || threadId === localStorage.getItem("newThreadId")) && /* @__PURE__ */ React4__namespace.createElement(
      QuickReplyButtons,
      {
        translations: translations2,
        isTyping,
        onQuickReply: handleQuickReply,
        faqItems
      }
    ), /* @__PURE__ */ React4__namespace.createElement(
      "button",
      {
        className: `w-full py-3 rounded-lg mb-4 ${phoneSubmitted && currentMessage.trim() && !isTyping ? "bg-[#2e67b4] text-white" : "bg-[#e4e9ee] text-[#bdc8d2]"}`,
        onClick: phoneSubmitted ? handleSendMessage : handlePhoneSubmit,
        disabled: (phoneSubmitted ? currentMessage.trim().length === 0 : phoneNumber.trim().length === 0) || isTyping
      },
      translations2.sendButton
    ), /* @__PURE__ */ React4__namespace.createElement(
      "button",
      {
        className: "w-full py-3 border border-[#2e67b4] text-[#2e67b4] rounded-lg",
        onClick: () => setIsCallbackModalOpen(true),
        disabled: isTyping
      },
      translations2.callbackRequestButton
    )))
  ), isCallbackModalOpen && /* @__PURE__ */ React4__namespace.createElement(
    CallbackModal,
    {
      isOpen: isCallbackModalOpen,
      onClose: () => setIsCallbackModalOpen(false),
      onSubmit: handleCallbackSubmit,
      translations: translations2
    }
  ));
}

exports.CallbackModal = CallbackModal;
exports.ChatbotInterface = ChatbotInterface;
exports.QuickReplyButtons = QuickReplyButtons;
exports.TypingIndicator = TypingIndicator;
exports.chatApi = chatApi;
exports.formatDate = formatDate;
exports.getTranslations = getTranslations;
//# sourceMappingURL=index.js.map
//# sourceMappingURL=index.js.map