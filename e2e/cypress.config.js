const { defineConfig } = require("cypress");

module.exports = defineConfig({
  e2e: {
    projectId: "8rjgtt",
    defaultCommandTimeout: 15000,
    requestTimeout: 20000,
    responseTimeout: 20000,
    pageLoadTimeout: 60000,
    setupNodeEvents(on, config) {
      // implement node event listeners here
    },
  },
});
