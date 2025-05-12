// This is a mock LLM service that would be replaced with a real API call in production
// In a real implementation, you would use the AI SDK to connect to an LLM provider

export async function generateLLMResponse(message: string): Promise<string> {
  // Simulate network delay and processing time
  const delay = Math.floor(Math.random() * 2000) + 1000; // Random delay between 1-3 seconds

  // Sample responses based on common insurance questions
  // In a real implementation, this would be replaced with an actual LLM call
  return new Promise((resolve) => {
    setTimeout(() => {
      // Simulate different response lengths and complexity
      if (message.toLowerCase().includes('полис')) {
        resolve(
          'Для продления полиса вам необходимо авторизоваться в личном кабинете на нашем сайте или в мобильном приложении. Там вы найдете раздел «Мои полисы», где можно выбрать нужный полис и нажать кнопку «Продлить». Также вы можете обратиться в ближайший офис нашей компании или позвонить в контакт-центр по номеру, указанному на сайте.',
        );
      } else if (message.toLowerCase().includes('договор')) {
        resolve(
          'Номер договора указан в верхней части вашего страхового полиса. Также вы можете найти его в личном кабинете на нашем сайте или в мобильном приложении в разделе «Мои полисы». Если у вас нет доступа к полису, вы можете обратиться в службу поддержки, предоставив ваши паспортные данные.',
        );
      } else if (message.toLowerCase().includes('заявление') || message.toLowerCase().includes('случай')) {
        resolve(
          'Для подачи заявления на страховой случай вам необходимо:\n\n1. Собрать документы, подтверждающие наступление страхового случая\n2. Заполнить форму заявления в личном кабинете или в мобильном приложении\n3. Прикрепить сканы или фотографии документов\n4. Отправить заявление\n\nПосле обработки заявления с вами свяжется наш специалист для уточнения деталей.',
        );
      } else if (message.toLowerCase().includes('спасибо')) {
        resolve(
          'Всегда рад помочь! Если у вас возникнут дополнительные вопросы, не стесняйтесь обращаться. Желаю вам хорошего дня!',
        );
      } else {
        resolve(
          'Спасибо за ваш вопрос. Я могу помочь вам с информацией о страховых полисах, процедуре подачи заявлений на страховые случаи, а также с другими вопросами, связанными со страхованием. Пожалуйста, уточните, какая именно информация вас интересует, и я постараюсь предоставить наиболее полный ответ.',
        );
      }
    }, delay);
  });
}
