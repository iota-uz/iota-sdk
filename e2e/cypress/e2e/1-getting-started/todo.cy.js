/// <reference types="cypress" />

// Welcome to Cypress!
//
// This spec file contains a variety of sample tests
// for a todo list app that are designed to demonstrate
// the power of writing tests in Cypress.
//
// To learn more about how Cypress works and
// what makes it such an awesome testing tool,
// please read our getting started guide:
// https://on.cypress.io/introduction-to-cypress

describe('example to-do app', () => {
    beforeEach(() => {
        cy.visit('http://localhost:3200/login')
        cy.get('[type=email]').type('test@gmail.com')
        cy.get('[type=password]').type('TestPass123!')
        cy.get('[type=submit]').click()
        cy.visit('http://localhost:3200/users')
    })

    afterEach(() => {
        cy.visit('http://localhost:3200/logout')
    })

    it('displays two todo items by default', () => {
        cy.get('[name=FirstName]').type('Test')
        cy.get('[name=LastName]').type('User')
        cy.get('[name=Email]').type('test1@gmail.com')
        cy.get('[name=Password]').type('TestPass123!')
        cy.get('[name=RoleID]').select('1')
        cy.get('[type=submit]').click()
        cy.visit('http://localhost:3200/logout')
        cy.visit('http://localhost:3200/users')
        cy.get('tbody tr').should('have.length', 2)

        cy.visit('http://localhost:3200/login')
        cy.get('[type=email]').type('test1@gmail.com')
        cy.get('[type=password]').type('TestPass123!')
        cy.get('[type=submit]').click()
        cy.visit('http://localhost:3200/users')

        cy.url().should('include', '/users')
        cy.get('tbody tr').should('have.length', 2)
    })
})
