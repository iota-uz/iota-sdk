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
  phoneInputLabel: string
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
  threadNotFoundMessage: string

  // Date formatting
  months: string[]
}

export const ru: Translations = {
  // Header
  chatbotTitle: 'Ai chat bot',
  chatbotSubtitle: 'Наш AI-бот готов помочь вам круглосуточно',

  // Welcome message
  welcomeGreeting: 'Привет! 👋',
  welcomeMessage: 'Я Али, ваш виртуальный помощник от Euroasia Insurance. Готов помочь разобраться в мире страхования, подобрать нужный полис или ответить на любые ваши вопросы.\n\n Я стараюсь предоставлять максимально точную информацию, но для важных решений рекомендую дополнительно уточнить детали на сайте или у наших консультантов номер 1147.\n Спрашивайте, не стесняйтесь! Чем могу быть полезен сегодня?',
  phoneRequestMessage:
    'Чтобы начать, пожалуйста, введите свой номер телефона — мы используем его для связи и сохранения истории обращений.',

  // Input placeholders
  phoneInputPlaceholder: '+ 998 (__) ___ __ __',
  phoneInputLabel: 'Введите ваш номер телефона',
  messageInputPlaceholder: 'Ваш запрос',

  // Buttons
  sendButton: 'Отправить',
  callbackRequestButton: 'Запрос обратного звонка',

  // Quick replies
  extendPolicyQuestion: 'Как продлить полис?',
  findContractNumberQuestion: 'Где найти номер договора?',
  submitClaimQuestion: 'Как подать заявление на страховой случай?',

  // Callback modal
  callbackModalTitle: 'Закажите обратный звонок',
  callbackModalSubtitle: 'Оставьте свой номер телефона, и наш специалист свяжется с вами в ближайшее время',
  callbackPhoneInputLabel: 'Введите номер телефона',
  dataPrivacyMessage: 'Мы не передаём ваши данные третьим лицам',
  dataProcessingConsent: 'Согласен(а) с обработкой персональных данных',
  backButton: 'Назад',
  requestCallButton: 'Заказать звонок',

  // Messages
  callbackConfirmation: 'Спасибо за запрос! Наш специалист свяжется с вами по номеру {phone} в ближайшее время.',
  errorLoadingMessages: 'Не удалось загрузить историю сообщений. Пожалуйста, попробуйте еще раз позже.',
  errorCreatingChat: 'Произошла ошибка при создании чата. Пожалуйста, попробуйте еще раз позже.',
  errorSendingMessage: 'Извините, произошла ошибка при отправке сообщения. Пожалуйста, попробуйте еще раз позже.',
  threadNotFoundMessage: 'Ваш чат был завершен или не найден. Начинаем новый чат.',

  // Date formatting
  months: [
    'Январь',
    'Февраль',
    'Март',
    'Апрель',
    'Май',
    'Июнь',
    'Июль',
    'Август',
    'Сентябрь',
    'Октябрь',
    'Ноябрь',
    'Декабрь',
  ],
};

export const oz: Translations = {
  // Header
  chatbotTitle: 'AI suhbat boti',
  chatbotSubtitle: 'Bizning AI-botimiz sizga 24/7 yordam berishga tayyor',

  // Welcome message
  welcomeGreeting: 'Assalomu alaykum! 👋',
  welcomeMessage:
    "Men Ali, Euroasia Insurance kompaniyasining virtual yordamchisiman. Sug'urta olamida yo'lingizni topishga, mos polis tanlashga va barcha savollaringizga javob berishga tayyorman. \n\n Imkon qadar aniq ma'lumot berishga harakat qilaman, ammo muhim qarorlar uchun saytimizdan yoki 1147 raqamli konsultantlarimizdan qo'shimcha maslahat olishingizni tavsiya qilaman. \nQo'rqmay so'rang! Bugun sizga qanday yordam kerak?",
  phoneRequestMessage:
    'Boshlash uchun, iltimos, telefon raqamingizni kiriting — biz undan aloqa va murojaat tarixini saqlash uchun foydalanamiz.',

  // Input placeholders
  phoneInputPlaceholder: '+ 998 (__) ___ __ __',
  phoneInputLabel: 'Telefon raqamingizni kiriting',
  messageInputPlaceholder: 'Sizning so\'rovingiz',

  // Buttons
  sendButton: 'Yuborish',
  callbackRequestButton: "Qayta qo'ng'iroq so'rovi",

  // Quick replies
  extendPolicyQuestion: 'Polisni qanday uzaytirish mumkin?',
  findContractNumberQuestion: 'Shartnoma raqamini qayerdan topish mumkin?',
  submitClaimQuestion: "Sug'urta hodisasi bo'yicha arizani qanday topshirish kerak?",

  // Callback modal
  callbackModalTitle: "Qayta qo'ng'iroq buyurtma qiling",
  callbackModalSubtitle:
    "Telefon raqamingizni qoldiring, va bizning mutaxassisimiz siz bilan yaqin vaqt ichida bog'lanadi",
  callbackPhoneInputLabel: 'Telefon raqamini kiriting',
  dataPrivacyMessage: "Biz sizning ma'lumotlaringizni uchinchi shaxslarga bermaydi",
  dataProcessingConsent: "Shaxsiy ma'lumotlarni qayta ishlashga roziman",
  backButton: 'Orqaga',
  requestCallButton: "Qo'ng'iroq buyurtma qilish",

  // Messages
  callbackConfirmation:
    "So'rov uchun rahmat! Mutaxassisimiz {phone} raqami orqali siz bilan yaqin vaqt ichida bog'lanadi.",
  errorLoadingMessages: "Xabarlar tarixini yuklab bo'lmadi. Iltimos, keyinroq qayta urinib ko'ring.",
  errorCreatingChat: "Chat yaratishda xatolik yuz berdi. Iltimos, keyinroq qayta urinib ko'ring.",
  errorSendingMessage: "Kechirasiz, xabar yuborishda xatolik yuz berdi. Iltimos, keyinroq qayta urinib ko'ring.",
  threadNotFoundMessage: 'Sizning chatingiz tugatilgan yoki topilmadi. Yangi chat boshlaymiz.',

  // Date formatting
  months: [
    'Yanvar',
    'Fevral',
    'Mart',
    'Aprel',
    'May',
    'Iyun',
    'Iyul',
    'Avgust',
    'Sentabr',
    'Oktabr',
    'Noyabr',
    'Dekabr',
  ],
};

export const uz: Translations = {
  // Header
  chatbotTitle: 'AI суҳбат боти',
  chatbotSubtitle: 'Бизнинг AI-ботимиз сизга 24/7 ёрдам беришга тайёр',

  // Welcome message
  welcomeGreeting: 'Ассалому алайкум! 👋',
  welcomeMessage:
    'Мен Али, Euroasia Insurance компаниясининг виртуал ёрдамчисиман. Суғурта оламида йўлингизни топишга, мос полис танлашга ва барча саволларингизга жавоб беришга тайёрман.\n\n Имкон қадар аниқ маълумот беришга ҳаракат қиламан, аммо муҳим қарорлар учун сайтимиздан ёки 1147 рақамли консультантларимиздан қўшимча маслаҳат олишингизни тавсия қиламан.\n Қўрқмай сўранг! Бугун сизга қандай ёрдам керак?',
  phoneRequestMessage:
    'Бошлаш учун, илтимос, телефон рақамингизни киритинг — биз ундан алоқа ва мурожаат тарихини сақлаш учун фойдаланамиз.',

  // Input placeholders
  phoneInputPlaceholder: '+ 998 (__) ___ __ __',
  phoneInputLabel: 'Телефон рақамингизни киритинг',
  messageInputPlaceholder: 'Сизнинг сўровингиз',

  // Buttons
  sendButton: 'Юбориш',
  callbackRequestButton: 'Қайта қўнғироқ сўрови',

  // Quick replies
  extendPolicyQuestion: 'Полисни қандай узайтириш мумкин?',
  findContractNumberQuestion: 'Шартнома рақамини қаердан топиш мумкин?',
  submitClaimQuestion: 'Суғурта ҳодисаси бўйича аризани қандай топшириш керак?',

  // Callback modal
  callbackModalTitle: 'Қайта қўнғироқ буюртма қилинг',
  callbackModalSubtitle:
    'Телефон рақамингизни қолдиринг, ва бизнинг мутахассисимиз сиз билан яқин вақт ичида боғланади',
  callbackPhoneInputLabel: 'Телефон рақамини киритинг',
  dataPrivacyMessage: 'Биз сизнинг маълумотларингизни учинчи шахсларга бермайди',
  dataProcessingConsent: 'Шахсий маълумотларни қайта ишлашга розиман',
  backButton: 'Орқага',
  requestCallButton: 'Қўнғироқ буюртма қилиш',

  // Messages
  callbackConfirmation: 'Сўров учун раҳмат! Мутахассисимиз {phone} рақами орқали сиз билан яқин вақт ичида боғланади.',
  errorLoadingMessages: 'Хабарлар тарихини юклаб бўлмади. Илтимос, кейинроқ қайта уриниб кўринг.',
  errorCreatingChat: 'Чат яратишда хатолик юз берди. Илтимос, кейинроқ қайта уриниб кўринг.',
  errorSendingMessage: 'Кечирасиз, хабар юборишда хатолик юз берди. Илтимос, кейинроқ қайта уриниб кўринг.',
  threadNotFoundMessage: 'Сизнинг чатингиз тугатилган ёки топилмади. Янги чат бошлаймиз.',

  // Date formatting
  months: [
    'Январь',
    'Февраль',
    'Март',
    'Апрель',
    'Май',
    'Июнь',
    'Июль',
    'Август',
    'Сентябрь',
    'Октябрь',
    'Ноябрь',
    'Декабрь',
  ],
};

export const en: Translations = {
  // Header
  chatbotTitle: 'AI chat bot',
  chatbotSubtitle: 'Our AI bot is ready to help you 24/7',

  // Welcome message
  welcomeGreeting: 'Hi there! 👋',
  welcomeMessage: "I'm Ali, your virtual assistant from Euroasia Insurance. I'm here to help you understand the insurance world, find the perfect policy, or answer any questions you might have.\n\n I do my best to provide accurate information, but for important decisions, I recommend double-checking details on our website or speaking with our consultants at 1147. Don't hesitate to ask anything!\n What can I help you with today?",
  phoneRequestMessage:
    'To get started, please enter your phone number — we use it for communication and to save your request history.',

  // Input placeholders
  phoneInputPlaceholder: '+ 998 (__) ___ __ __',
  phoneInputLabel: 'Enter your phone number',
  messageInputPlaceholder: 'Your request',

  // Buttons
  sendButton: 'Send',
  callbackRequestButton: 'Request a callback',

  // Quick replies
  extendPolicyQuestion: 'How to extend my policy?',
  findContractNumberQuestion: 'Where to find my contract number?',
  submitClaimQuestion: 'How to submit an insurance claim?',

  // Callback modal
  callbackModalTitle: 'Request a callback',
  callbackModalSubtitle: 'Leave your phone number, and our specialist will contact you shortly',
  callbackPhoneInputLabel: 'Enter phone number',
  dataPrivacyMessage: "We don't share your data with third parties",
  dataProcessingConsent: 'I agree to the processing of personal data',
  backButton: 'Back',
  requestCallButton: 'Request call',

  // Messages
  callbackConfirmation: 'Thank you for your request! Our specialist will contact you at {phone} shortly.',
  errorLoadingMessages: 'Failed to load message history. Please try again later.',
  errorCreatingChat: 'An error occurred while creating the chat. Please try again later.',
  errorSendingMessage: 'Sorry, an error occurred while sending the message. Please try again later.',
  threadNotFoundMessage: 'Your chat has been ended or not found. Starting a new chat.',

  // Date formatting
  months: [
    'January',
    'February',
    'March',
    'April',
    'May',
    'June',
    'July',
    'August',
    'September',
    'October',
    'November',
    'December',
  ],
};

export const translations: Record<string, Translations> = {
  ru,
  oz,
  uz,
  en,
};

export function getTranslations(locale: string): Translations {
  return translations[locale] || en;
}
