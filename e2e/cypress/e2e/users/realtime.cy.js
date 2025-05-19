/// <reference types="cypress" />

describe("user realtime behavior", () => {
    before(() => {
        cy.task("resetDatabase");
        cy.task("seedDatabase");
    });

    beforeEach(() => {
        cy.viewport(1280, 720);
    });

    afterEach(() => {
        cy.logout();
    });

    it("updates user table in realtime when a user is created, edited, and deleted", () => {
        cy.login("test@gmail.com", "TestPass123!");
        cy.visit("http://localhost:3200/users");
        cy.url().should("eq", "http://localhost:3200/users");

        // Get initial row count
        cy.get("tbody tr").then($initialRows => {
            const initialRowCount = $initialRows.length;

            // This simulates adding a user through a different session
            cy.request({
                method: "POST",
                url: "http://localhost:3200/users",
                form: true,
                body: {
                    FirstName: "Realtime",
                    LastName: "Test",
                    MiddleName: "Mid",
                    Email: "realtime@gmail.com",
                    Phone: "+14155551234",
                    Password: "TestPass123!",
                    Language: "en",
                    RoleIDs: "1",
                }
            });

            // Verify user was added in realtime
            cy.get("tbody tr").should("have.length", initialRowCount + 1);
            cy.contains("tbody tr", "Realtime Test").should("exist");

            // Get the user ID from the href attribute of the edit link
            cy.contains("tbody tr", "Realtime Test").find("td a").invoke("attr", "href").then((href) => {
                const userId = href.split("/").pop();

                // Edit the user through a direct request (staying on the users page)
                cy.request({
                    method: "POST",
                    url: `http://localhost:3200/users/${userId}`,
                    form: true,
                    body: {
                        FirstName: "RealtimeUpdated",
                        LastName: "TestUpdated",
                        MiddleName: "Mid",
                        Email: "realtime@gmail.com",
                        Phone: "+14155559876",
                        Language: "en",
                        RoleIDs: "1",
                    }
                });

                // Verify user was updated in the table without refreshing
                cy.contains("tbody tr", "RealtimeUpdated TestUpdated").should("exist");
                cy.contains("tbody tr", "Realtime Test").should("not.exist");
                cy.get("tbody tr").should("have.length", initialRowCount + 1);

                // Delete the user through a direct request
                cy.request({
                    method: "DELETE",
                    url: `http://localhost:3200/users/${userId}`
                });

                // Verify user was removed from the table without refreshing
                cy.contains("tbody tr", "RealtimeUpdated TestUpdated").should("not.exist");
                cy.get("tbody tr").should("have.length", initialRowCount);
            });
        });
    });
});
