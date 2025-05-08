// Define the structure of our translations
export interface Translations {
  // Header
  chatbotTitle: string
  chatbotSubtitle: string

  // Welcome message
  welcomeGreeting: string
  welcomeMessage: string
  phoneRequestMessage: string

  // Input placeholders
  phoneInputPlaceholder: string
  messageInputPlaceholder: string

  // Buttons
  sendButton: string
  callbackRequestButton: string

  // Quick replies
  extendPolicyQuestion: string
  findContractNumberQuestion: string
  submitClaimQuestion: string

  // Callback modal
  callbackModalTitle: string
  callbackModalSubtitle: string
  callbackPhoneInputLabel: string
  dataPrivacyMessage: string
  dataProcessingConsent: string
  backButton: string
  requestCallButton: string

  // Messages
  callbackConfirmation: string
  errorLoadingMessages: string
  errorCreatingChat: string
  errorSendingMessage: string

  // Date formatting
  months: string[]
}

// Russian translations (default)
export const ru: Translations = {
  // Header
  chatbotTitle: "Ai chat bot",
  chatbotSubtitle: "Наш AI-бот готов помочь вам круглосуточно",

  // Welcome message
  welcomeGreeting: "Привет! Я виртуальный помощник Euroasia Insurance 👋",
  welcomeMessage: "Готов помочь вам с оформлением полиса, расчетом стоимости и любыми вопросами.",
  phoneRequestMessage:
    "Чтобы начать, пожалуйста, введите свой номер телефона — мы используем его для связи и сохранения истории обращений.",

  // Input placeholders
  phoneInputPlaceholder: "+ 998 (__) ___ __ __",
  messageInputPlaceholder: "Сообщения",

  // Buttons
  sendButton: "Отправить",
  callbackRequestButton: "Запрос обратного звонка",

  // Quick replies
  extendPolicyQuestion: "Как продлить полис?",
  findContractNumberQuestion: "Где найти номер договора?",
  submitClaimQuestion: "Как подать заявление на страховой случай?",

  // Callback modal
  callbackModalTitle: "Закажите обратный звонок",
  callbackModalSubtitle: "Оставьте свой номер телефона, и наш специалист свяжется с вами в ближайшее время",
  callbackPhoneInputLabel: "Введите номер телефона",
  dataPrivacyMessage: "Мы не передаём ваши данные третьим лицам",
  dataProcessingConsent: "Согласен(а) с обработкой персональных данных",
  backButton: "Назад",
  requestCallButton: "Заказать звонок",

  // Messages
  callbackConfirmation: "Спасибо за запрос! Наш специалист свяжется с вами по номеру {phone} в ближайшее время.",
  errorLoadingMessages: "Не удалось загрузить историю сообщений. Пожалуйста, попробуйте еще раз позже.",
  errorCreatingChat: "Произошла ошибка при создании чата. Пожалуйста, попробуйте еще раз позже.",
  errorSendingMessage: "Извините, произошла ошибка при отправке сообщения. Пожалуйста, попробуйте еще раз позже.",

  // Date formatting
  months: [
    "Январь",
    "Февраль",
    "Март",
    "Апрель",
    "Май",
    "Июнь",
    "Июль",
    "Август",
    "Сентябрь",
    "Октябрь",
    "Ноябрь",
    "Декабрь",
  ],
}

// Uzbek translations
export const uz: Translations = {
  // Header
  chatbotTitle: "AI suhbat boti",
  chatbotSubtitle: "Bizning AI-botimiz sizga 24/7 yordam berishga tayyor",

  // Welcome message
  welcomeGreeting: "Salom! Men Euroasia Insurance virtual yordamchisiman 👋",
  welcomeMessage:
    "Sug'urta polisini rasmiylashtirish, narxni hisoblash va har qanday savollar bo'yicha yordam berishga tayyorman.",
  phoneRequestMessage:
    "Boshlash uchun, iltimos, telefon raqamingizni kiriting — biz undan aloqa va murojaat tarixini saqlash uchun foydalanamiz.",

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
  callbackModalSubtitle:
    "Telefon raqamingizni qoldiring, va bizning mutaxassisimiz siz bilan yaqin vaqt ichida bog'lanadi",
  callbackPhoneInputLabel: "Telefon raqamini kiriting",
  dataPrivacyMessage: "Biz sizning ma'lumotlaringizni uchinchi shaxslarga bermaydi",
  dataProcessingConsent: "Shaxsiy ma'lumotlarni qayta ishlashga roziman",
  backButton: "Orqaga",
  requestCallButton: "Qo'ng'iroq buyurtma qilish",

  // Messages
  callbackConfirmation:
    "So'rov uchun rahmat! Mutaxassisimiz {phone} raqami orqali siz bilan yaqin vaqt ichida bog'lanadi.",
  errorLoadingMessages: "Xabarlar tarixini yuklab bo'lmadi. Iltimos, keyinroq qayta urinib ko'ring.",
  errorCreatingChat: "Chat yaratishda xatolik yuz berdi. Iltimos, keyinroq qayta urinib ko'ring.",
  errorSendingMessage: "Kechirasiz, xabar yuborishda xatolik yuz berdi. Iltimos, keyinroq qayta urinib ko'ring.",

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
    "Dekabr",
  ],
}

// English translations
export const en: Translations = {
  // Header
  chatbotTitle: "AI chat bot",
  chatbotSubtitle: "Our AI bot is ready to help you 24/7",

  // Welcome message
  welcomeGreeting: "Hello! I'm the virtual assistant of Euroasia Insurance 👋",
  welcomeMessage: "I'm ready to help you with policy registration, cost calculation, and any questions you may have.",
  phoneRequestMessage:
    "To get started, please enter your phone number — we use it for communication and to save your request history.",

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
    "December",
  ],
}

// Map of all available translations
export const translations: Record<string, Translations> = {
  ru,
  uz,
  en,
}

// Function to get translations for a specific locale
export function getTranslations(locale: string): Translations {
  return translations[locale] || en // Fallback to English if locale not found
}
